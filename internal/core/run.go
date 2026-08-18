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
	seq := s.nextSeq()
	s.emit(ch, EventSessionStarted, &SessionStartedPayload{Prompt: text})

	// Add the user message and persist it.
	msg := Message{Role: RoleUser, Content: text}
	s.Messages = append(s.Messages, msg)
	s.emit(ch, EventUserMessage, &UserMessagePayload{Text: text})

	var usage Usage
	changed := map[string]bool{}

	// Record the worktree state at session start so changed files can be
	// determined relative to it.
	baseline := snapshotWorktree(s.cfg.WorkingDir)

	turns := 0
	for {
		if s.cfg.MaxTurns > 0 && turns >= s.cfg.MaxTurns {
			s.fail(ch, seq, &Error{Code: "max_turns", Message: fmt.Sprintf("max turns (%d) reached", s.cfg.MaxTurns)})
			return
		}
		turns++

		s.emit(ch, EventModelStarted, &ModelStartedPayload{Turn: s.Turn + 1})

		var calls []ToolCall
		var textBuf strings.Builder
		var turnUsage Usage
		err := s.cfg.Provider.Complete(ctx, s, s.cfg.Tools, func(pe ProviderEvent) {
			if pe.Text != "" {
				textBuf.WriteString(pe.Text)
				s.emit(ch, EventTextDelta, &TextDeltaPayload{Text: pe.Text})
			}
			if pe.ToolCall != nil {
				calls = append(calls, *pe.ToolCall)
			}
			if pe.Usage != nil {
				turnUsage = *pe.Usage
			}
			if pe.Err != nil {
				// Surface the provider error as a failure.
				s.fail(ch, seq, pe.Err)
			}
		})
		if err != nil {
			s.fail(ch, seq, &Error{Code: "provider", Message: err.Error()})
			return
		}

		usage = addUsage(usage, turnUsage)
		s.emit(ch, EventUsage, &UsagePayload{Usage: usage})
		s.emit(ch, EventModelCompleted, &ModelCompletedPayload{Turn: s.Turn + 1, Usage: &turnUsage})
		s.Turn++

		// Record the assistant message with its text and tool calls.
		assistant := Message{Role: RoleAssistant, Content: textBuf.String(), ToolCalls: calls}
		s.Messages = append(s.Messages, assistant)

		if len(calls) == 0 {
			// The model finished; build the result and complete.
			res := s.buildResult(textBuf.String(), usage, changed, baseline)
			s.emit(ch, EventSessionCompleted, &SessionCompletedPayload{Result: res})
			return
		}

		// Run each tool call and feed its result back.
		for _, call := range calls {
			s.emit(ch, EventToolStarted, &ToolStartedPayload{CallID: call.ID, Name: call.Name, Input: call.Input})
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

			// Store large outputs as artifacts.
			if len(output) > s.cfg.MaxInline {
				art := s.storeOutput(call.Name, output)
				if art != nil {
					s.emit(ch, EventArtifactCreated, &ArtifactCreatedPayload{Artifact: art})
					output = artifactPreview(art, s.cfg.MaxPreview)
				}
			}

			// Track changed files for edit/bash tools.
			if call.Name == "edit" || call.Name == "bash" {
				recordChangedFiles(changed, s.cfg.WorkingDir, call.Name, call.Input)
			}

			// Verification purpose: record verification status. The bash
			// tool reports a non-zero exit as a string result with a nil
			// error, so the status is parsed from the output.
			if call.Name == "bash" && isVerification(call.Input) {
				ver := &Verification{Command: verificationCommand(call.Input), Status: "failed", ExitCode: 1}
				if runErr == nil {
					code := exitCodeFromOutput(output)
					if code == 0 {
						ver.Status = "passed"
						ver.ExitCode = 0
					} else {
						ver.ExitCode = code
					}
				}
				s.emit(ch, EventVerification, &VerificationPayload{Verification: ver})
			}

			s.emit(ch, EventToolFinished, &ToolFinishedPayload{CallID: call.ID, Name: call.Name, Output: output, Error: runErr})
			s.Messages = append(s.Messages, Message{Role: RoleTool, ToolCallID: call.ID, Content: output})
		}
	}
}

// fail emits a session.failed event and stops the run.
func (s *Session) fail(ch chan<- Event, _ int, e *Error) {
	s.emit(ch, EventSessionFailed, &SessionFailedPayload{Error: e})
}

// emit persists an event and publishes it to the channel.
func (s *Session) emit(ch chan<- Event, typ string, payload any) {
	ev := &Event{
		ID:        newID("evt"),
		Seq:       s.nextSeq(),
		SessionID: s.ID,
		Type:      typ,
		Time:      time.Now().UTC(),
		Payload:   payload,
	}
	if s.store != nil {
		_ = s.store.AppendEvent(s.ID, ev)
	}
	s.events = append(s.events, ev)
	ch <- *ev
}

// nextSeq returns the next durable sequence number.
func (s *Session) nextSeq() int {
	return len(s.events) + 1
}

// storeOutput stores a large tool output as an artifact and returns it.
func (s *Session) storeOutput(_ string, output string) *Artifact {
	art := &Artifact{
		ID:        newID("art"),
		SessionID: s.ID,
		Size:      int64(len(output)),
		MediaType: "text/plain; charset=utf-8",
	}
	if s.store != nil {
		if err := s.store.StoreArtifact(s.ID, art.ID, []byte(output)); err != nil {
			return nil
		}
	}
	return art
}

// artifactPreview builds the head/tail preview returned in a tool result.
func artifactPreview(art *Artifact, maxPreview int) string {
	if maxPreview <= 0 {
		maxPreview = 8 * 1024
	}
	head := art.Preview
	if head == "" {
		head = "(artifact " + art.ID + ")"
	}
	// The stored preview is populated by the store; fall back to a compact
	// reference with the ID, size, and media type.
	return fmt.Sprintf("[artifact %s size=%d media=%s]\n%s", art.ID, art.Size, art.MediaType, head)
}

// buildResult assembles the structured result for a completed prompt.
func (s *Session) buildResult(text string, usage Usage, changed map[string]bool, baseline map[string]bool) *Result {
	res := &Result{
		Status:               "completed",
		Text:                 text,
		Usage:                usage,
		ChangedFilesComplete: true,
	}
	changedFiles, complete := diffWorktree(s.cfg.WorkingDir, baseline)
	res.ChangedFiles = changedFiles
	res.ChangedFilesComplete = complete
	if len(changed) > 0 {
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
