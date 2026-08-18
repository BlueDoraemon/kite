package core

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"
)

// run drives the agent loop for a single prompt. It emits durable,
// sequence-numbered events and publishes them to the caller's channel.
func (s *Session) run(ctx context.Context, text string, ch chan<- Event) {
	for len(s.pendingInterrupts) > 0 {
		call := s.pendingInterrupts[0]
		if err := s.emit(ch, EventInterruptedTool, &InterruptedToolPayload{Call: &call}); err != nil {
			return
		}
		s.pendingInterrupts = s.pendingInterrupts[1:]
	}
	if err := s.emit(ch, EventSessionStarted, &SessionStartedPayload{Prompt: text}); err != nil {
		return
	}

	// Add the user message and persist it.
	msg := Message{Role: RoleUser, Content: text}
	s.Messages = append(s.Messages, msg)
	if err := s.emit(ch, EventUserMessage, &UserMessagePayload{Text: text}); err != nil {
		return
	}

	var usage Usage
	changed := map[string]bool{}

	// Record the worktree state at prompt start so changed files can be
	// determined relative to it.
	baseline := snapshotWorktree(s.cfg.WorkingDir)

	turns := 0
	for {
		if s.cfg.MaxTurns > 0 && turns >= s.cfg.MaxTurns {
			_ = s.fail(ch, &Error{Code: "max_turns", Message: fmt.Sprintf("max turns (%d) reached", s.cfg.MaxTurns)})
			return
		}
		turns++

		if err := s.emit(ch, EventModelStarted, &ModelStartedPayload{Turn: s.Turn + 1}); err != nil {
			return
		}

		var calls []ToolCall
		var textBuf strings.Builder
		var turnUsage Usage
		var providerErr *Error
		var eventErr error
		err := s.cfg.Provider.Complete(ctx, s, s.cfg.Tools, func(pe ProviderEvent) {
			if eventErr != nil || providerErr != nil {
				return
			}
			if pe.Text != "" {
				textBuf.WriteString(pe.Text)
				eventErr = s.emit(ch, EventTextDelta, &TextDeltaPayload{Text: pe.Text})
			}
			if pe.ToolCall != nil {
				calls = append(calls, *pe.ToolCall)
			}
			if pe.Usage != nil {
				turnUsage = *pe.Usage
			}
			if pe.Err != nil {
				providerErr = pe.Err
			}
		})
		if eventErr != nil {
			return
		}
		if providerErr != nil {
			_ = s.fail(ch, providerErr)
			return
		}
		if err != nil {
			_ = s.fail(ch, &Error{Code: "provider", Message: err.Error()})
			return
		}

		usage = addUsage(usage, turnUsage)
		if err := s.emit(ch, EventUsage, &UsagePayload{Usage: usage}); err != nil {
			return
		}
		if err := s.emit(ch, EventModelCompleted, &ModelCompletedPayload{Turn: s.Turn + 1, Usage: &turnUsage}); err != nil {
			return
		}
		s.Turn++

		// Record the assistant message with its text and tool calls.
		assistant := Message{Role: RoleAssistant, Content: textBuf.String(), ToolCalls: calls}
		s.Messages = append(s.Messages, assistant)

		if len(calls) == 0 {
			// The model finished; build the result and complete.
			res := s.buildResult(textBuf.String(), usage, changed, baseline)
			_ = s.emit(ch, EventSessionCompleted, &SessionCompletedPayload{Result: res})
			return
		}

		// Run each tool call and feed its result back.
		for _, call := range calls {
			if err := s.emit(ch, EventToolStarted, &ToolStartedPayload{CallID: call.ID, Name: call.Name, Input: call.Input}); err != nil {
				return
			}
			beforeTool := snapshotWorktree(s.cfg.WorkingDir)
			tool := findTool(s.cfg.Tools, call.Name)
			var output string
			var runErr *Error
			if tool == nil {
				runErr = &Error{Code: "unknown_tool", Message: fmt.Sprintf("unknown tool %q", call.Name)}
			} else {
				out, err := tool.Run(ctx, call.Input)
				if err != nil {
					runErr = &Error{Code: "tool_error", Message: err.Error()}
					output = "error: " + err.Error()
				} else {
					output = out
				}
			}

			rawOutput := output
			var artifactID string
			if len(output) > s.cfg.MaxInline {
				art, err := s.storeOutput(output)
				if err != nil {
					_ = s.fail(ch, &Error{Code: "artifact_store", Message: err.Error()})
					return
				}
				if err := s.emit(ch, EventArtifactCreated, &ArtifactCreatedPayload{Artifact: art}); err != nil {
					return
				}
				artifactID = art.ID
				output = artifactPreview(art)
			}

			// Track changed files for edit/bash tools.
			if call.Name == "edit" || call.Name == "bash" {
				recordChangedFiles(changed, s.cfg.WorkingDir, call.Name, call.Input)
			}

			// Close the tool hunk before emitting any derived verification
			// status. Consumers can then render verification as a seal over a
			// completed tool result rather than as activity inside the tool.
			if err := s.emit(ch, EventToolFinished, &ToolFinishedPayload{CallID: call.ID, Name: call.Name, Output: output, Error: runErr}); err != nil {
				return
			}
			s.Messages = append(s.Messages, Message{Role: RoleTool, ToolCallID: call.ID, Content: output})

			// Verification purpose: record verification status. The bash
			// tool reports a non-zero exit as a string result with a nil
			// error, so the status is parsed from the output.
			verificationCall := call.Name == "bash" && isVerification(call.Input)
			if verificationCall {
				ver := &Verification{Command: verificationCommand(call.Input), Status: "failed", ExitCode: 1}
				if runErr == nil {
					code := exitCodeFromOutput(rawOutput)
					if code == 0 {
						ver.Status = "passed"
						ver.ExitCode = 0
					} else {
						ver.ExitCode = code
					}
				}
				if artifactID != "" {
					ver.Artifacts = []string{artifactID}
				}
				s.latestVerification = ver
				if err := s.emit(ch, EventVerification, &VerificationPayload{Verification: ver}); err != nil {
					return
				}
			} else if s.latestVerification != nil && worktreeChanged(beforeTool, snapshotWorktree(s.cfg.WorkingDir)) {
				stale := *s.latestVerification
				stale.Stale = true
				s.latestVerification = &stale
				if err := s.emit(ch, EventVerification, &VerificationPayload{Verification: &stale}); err != nil {
					return
				}
			}
		}
	}
}

// fail emits a session.failed event and stops the run.
func (s *Session) fail(ch chan<- Event, e *Error) error {
	return s.emit(ch, EventSessionFailed, &SessionFailedPayload{Error: e})
}

// emit persists an event and publishes it to the channel.
func (s *Session) emit(ch chan<- Event, typ string, payload any) error {
	ev := &Event{
		ID:        newID("evt"),
		Seq:       s.nextSeq(),
		SessionID: s.ID,
		Type:      typ,
		Time:      time.Now().UTC(),
		Payload:   payload,
	}
	if s.store != nil {
		if err := s.store.AppendEvent(s.ID, ev); err != nil {
			s.persistenceErr = err
			return err
		}
	}
	s.events = append(s.events, ev)
	ch <- *ev
	return nil
}

// nextSeq returns the next durable sequence number.
func (s *Session) nextSeq() int {
	return len(s.events) + 1
}

// storeOutput stores a large tool output as an artifact and returns it.
func (s *Session) storeOutput(output string) (*Artifact, error) {
	art := &Artifact{
		ID:        newID("art"),
		SessionID: s.ID,
		Size:      int64(len(output)),
		MediaType: "text/plain; charset=utf-8",
		Preview:   outputPreview(output, s.cfg.MaxPreview),
	}
	if s.store != nil {
		if err := s.store.StoreArtifact(s.ID, art.ID, []byte(output)); err != nil {
			return nil, err
		}
	}
	return art, nil
}

// artifactPreview builds the head/tail preview returned in a tool result.
func artifactPreview(art *Artifact) string {
	return fmt.Sprintf("[artifact %s size=%d media=%s]\n%s", art.ID, art.Size, art.MediaType, art.Preview)
}

func outputPreview(output string, limit int) string {
	if limit <= 0 {
		limit = 8 * 1024
	}
	if len(output) <= limit {
		return output
	}
	const marker = "\n... preview truncated ...\n"
	if limit <= len(marker) {
		return output[:limit]
	}
	contentLimit := limit - len(marker)
	head := contentLimit / 2
	tail := contentLimit - head
	return output[:head] + marker + output[len(output)-tail:]
}

// buildResult assembles the structured result for a completed prompt.
func (s *Session) buildResult(text string, usage Usage, changed map[string]bool, baseline worktreeSnapshot) *Result {
	res := &Result{
		Status:               "completed",
		Text:                 text,
		Usage:                usage,
		ChangedFilesComplete: true,
	}
	if s.latestVerification != nil {
		verification := *s.latestVerification
		res.Verification = &verification
		if verification.Status == "failed" || verification.Stale {
			res.Status = "failed"
		}
	}
	changedFiles, complete := diffWorktree(s.cfg.WorkingDir, baseline)
	res.ChangedFiles = changedFiles
	res.ChangedFilesComplete = complete
	if !complete && len(changed) > 0 {
		res.ChangedFiles = mergeChanged(res.ChangedFiles, changed)
	}
	return res
}

// addUsage sums two usage records.
func addUsage(a, b Usage) Usage {
	return Usage{
		PromptTokens:     a.PromptTokens + b.PromptTokens,
		CompletionTokens: a.CompletionTokens + b.CompletionTokens,
		TotalTokens:      a.TotalTokens + b.TotalTokens,
	}
}

// mergeChanged merges two changed-file sets deterministically.
func mergeChanged(a []string, b map[string]bool) []string {
	set := map[string]bool{}
	for _, f := range a {
		set[f] = true
	}
	for f := range b {
		set[f] = true
	}
	out := make([]string, 0, len(set))
	for f := range set {
		out = append(out, f)
	}
	sort.Strings(out)
	return out
}

// findTool returns the registered tool with the given name, or nil.
func findTool(tools []Tool, name string) Tool {
	for _, t := range tools {
		if t.Name() == name {
			return t
		}
	}
	return nil
}
