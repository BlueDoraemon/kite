package core

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

// Session is the public handle to a Kite agent session. It is safe for
// concurrent use except that only one prompt may be active at a time.
type Session struct {
	mu sync.Mutex

	ID       string
	Model    string
	Messages []Message
	StartedAt time.Time
	Turn     int
	Interrupted []ToolCall

	cfg    Config
	store  SessionStore
	events []*Event

	active bool
}

// NewSession creates a new session from cfg. Setup errors (nil provider,
// nil model, an unusable data directory) are returned directly; runtime
// failures during a prompt are delivered as session.failed events.
func NewSession(cfg Config) (*Session, error) {
	if cfg.Provider == nil {
		return nil, fmt.Errorf("kite: nil provider")
	}
	if cfg.Model == "" {
		return nil, fmt.Errorf("kite: empty model")
	}
	if cfg.MaxInline == 0 {
		cfg.MaxInline = 16 * 1024
	}
	if cfg.MaxPreview == 0 {
		cfg.MaxPreview = 8 * 1024
	}
	if cfg.Tools == nil {
		cfg.Tools = builtinTools(cfg.WorkingDir)
	}
	store, err := openStore(cfg.DataDir)
	if err != nil {
		return nil, err
	}
	s := &Session{
		ID:        newID("sess"),
		Model:     cfg.Model,
		StartedAt: time.Now().UTC(),
		cfg:       cfg,
		store:     store,
	}
	return s, nil
}

// LoadSession loads a persisted session by id. The session is reconstructed
// from its durable event log, so it can be resumed exactly where it left off.
func LoadSession(cfg Config, id string) (*Session, error) {
	if id == "" {
		return nil, fmt.Errorf("kite: empty session id")
	}
	if cfg.MaxInline == 0 {
		cfg.MaxInline = 16 * 1024
	}
	if cfg.MaxPreview == 0 {
		cfg.MaxPreview = 8 * 1024
	}
	if cfg.Tools == nil {
		cfg.Tools = builtinTools(cfg.WorkingDir)
	}
	store, err := openStore(cfg.DataDir)
	if err != nil {
		return nil, err
	}
	evs, err := store.LoadEvents(id)
	if err != nil {
		return nil, err
	}
	s := &Session{
		ID:    id,
		Model: cfg.Model,
		cfg:   cfg,
		store: store,
	}
	replaySession(s, evs)
	return s, nil
}

// replaySession reconstructs a session's in-memory state from its durable
// event log. Only complete durable turns are replayed; interrupted tool
// calls are recorded and never re-run.
func replaySession(s *Session, evs []*Event) {
	for _, ev := range evs {
		switch ev.Type {
		case EventSessionStarted:
			s.StartedAt = ev.Time
		case EventUserMessage:
			if p, ok := ev.Payload.(*UserMessagePayload); ok {
				s.Messages = append(s.Messages, Message{Role: RoleUser, Content: p.Text})
			}
		case EventTextDelta:
			if p, ok := ev.Payload.(*TextDeltaPayload); ok {
				if n := len(s.Messages); n > 0 && s.Messages[n-1].Role == RoleAssistant {
					s.Messages[n-1].Content += p.Text
				} else {
					s.Messages = append(s.Messages, Message{Role: RoleAssistant, Content: p.Text})
				}
			}
		case EventToolStarted:
			if p, ok := ev.Payload.(*ToolStartedPayload); ok {
				// Mark the assistant message as carrying this call.
				if n := len(s.Messages); n > 0 && s.Messages[n-1].Role == RoleAssistant {
					s.Messages[n-1].ToolCalls = append(s.Messages[n-1].ToolCalls, ToolCall{ID: p.CallID, Name: p.Name, Input: p.Input})
				}
			}
		case EventToolFinished:
			if p, ok := ev.Payload.(*ToolFinishedPayload); ok {
				content := p.Output
				if p.Error != nil {
					content = "error: " + p.Error.Message
				}
				s.Messages = append(s.Messages, Message{Role: RoleTool, ToolCallID: p.CallID, Content: content})
			}
		case EventModelCompleted:
			if p, ok := ev.Payload.(*ModelCompletedPayload); ok {
				s.Turn = p.Turn
			}
		case EventInterruptedTool:
			if p, ok := ev.Payload.(*InterruptedToolPayload); ok && p.Call != nil {
				s.Interrupted = append(s.Interrupted, *p.Call)
			}
		}
	}
}

// Prompt runs a prompt on the session and returns a channel of events. Only
// one prompt may be active per session; calling Prompt again while one is
// running returns an error. Runtime failures are emitted as session.failed
// events on the channel, not returned as errors.
func (s *Session) Prompt(ctx context.Context, text string) (<-chan Event, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.active {
		return nil, fmt.Errorf("kite: a prompt is already active on this session")
	}
	s.active = true

	ch := make(chan Event, 64)
	go func() {
		defer func() { s.mu.Lock(); s.active = false; s.mu.Unlock() }()
		s.run(ctx, text, ch)
		close(ch)
	}()
	return ch, nil
}

// BuildContext returns the deterministic context for the session: fixed
// system instructions, repository instructions, completed messages, and
// bounded tool previews. It is exported so consumers can inspect exactly what
// a model will see.
func (s *Session) BuildContext() []Message {
	return buildContext(s)
}

// DataDir returns the data directory the session persists to.
func (s *Session) DataDir() string { return s.cfg.DataDir }

// Store returns the session store, for advanced consumers.
func (s *Session) Store() SessionStore { return s.store }

// newID returns a globally unique prefixed identifier.
func newID(prefix string) string {
	var b [12]byte
	if _, err := rand.Read(b[:]); err != nil {
		// Fall back to a time-based id; rand failure is essentially
		// impossible on supported platforms.
		return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
	}
	return prefix + "_" + hex.EncodeToString(b[:])
}
