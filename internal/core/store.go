package core

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// defaultDataDir returns the platform data directory for Kite.
func defaultDataDir() string {
	if v := os.Getenv("KITE_DATA_DIR"); v != "" {
		return v
	}
	return platformDataDir()
}

// openStore opens (and creates, if needed) the session store rooted at
// dataDir. An empty dataDir selects the platform default.
func openStore(dataDir string) (*FileStore, error) {
	if dataDir == "" {
		dataDir = defaultDataDir()
	}
	if err := os.MkdirAll(filepath.Join(dataDir, "sessions"), 0o700); err != nil {
		return nil, fmt.Errorf("kite: create data dir: %w", err)
	}
	if err := os.MkdirAll(filepath.Join(dataDir, "artifacts"), 0o700); err != nil {
		return nil, fmt.Errorf("kite: create artifacts dir: %w", err)
	}
	return &FileStore{dir: dataDir}, nil
}

// OpenStore opens the default session store. It is exported for consumers
// that need to list or inspect persisted sessions.
func OpenStore() (*FileStore, error) {
	return openStore("")
}

// FileStore persists sessions and artifacts to the filesystem.
type FileStore struct {
	dir string
}

// sessionPath returns the JSONL path for a session.
func (f *FileStore) sessionPath(id string) string {
	return filepath.Join(f.dir, "sessions", id+".jsonl")
}

// artifactPath returns the artifact path for a session and artifact.
func (f *FileStore) artifactPath(sessionID, artifactID string) string {
	return filepath.Join(f.dir, "artifacts", sessionID, artifactID)
}

// AppendEvent durably appends an event to the session log.
func (f *FileStore) AppendEvent(sessionID string, ev *Event) error {
	path := f.sessionPath(sessionID)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	line, err := json.Marshal(ev)
	if err != nil {
		return err
	}
	fh, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	defer fh.Close()
	if _, err := fh.Write(append(line, '\n')); err != nil {
		return err
	}
	return fh.Sync()
}

// LoadEvents returns all durable events for a session in order. Truncated or
// malformed trailing records are ignored so a crash mid-write cannot corrupt
// the log.
func (f *FileStore) LoadEvents(sessionID string) ([]*Event, error) {
	path := f.sessionPath(sessionID)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("kite: session %s not found", sessionID)
		}
		return nil, err
	}
	var evs []*Event
	for _, line := range splitLines(data) {
		if len(line) == 0 {
			continue
		}
		var ev Event
		if err := json.Unmarshal(line, &ev); err != nil {
			// Stop at the first malformed record; the log is truncated.
			break
		}
		ev.Payload = decodePayload(ev.Type, ev.Payload)
		evs = append(evs, &ev)
	}
	return evs, nil
}

// decodePayload converts a JSON-decoded event payload (a map[string]any) back
// into the typed payload struct for the event type. Unknown types keep the
// raw map, which consumers must tolerate per the compatibility policy.
// (consumers must ignore unknown JSON fields).
func decodePayload(typ string, raw any) any {
	if raw == nil {
		return nil
	}
	// Already typed (in-memory events) pass through.
	switch raw.(type) {
	case *SessionStartedPayload, *SessionCompletedPayload, *SessionFailedPayload,
		*ModelStartedPayload, *ModelCompletedPayload, *TextDeltaPayload,
		*ToolStartedPayload, *ToolFinishedPayload, *ArtifactCreatedPayload,
		*UserMessagePayload, *UsagePayload, *ResumePayload, *VerificationPayload,
		*InterruptedToolPayload:
		return raw
	}
	data, err := json.Marshal(raw)
	if err != nil {
		return raw
	}
	var out any
	switch typ {
	case EventSessionStarted:
		out = &SessionStartedPayload{}
	case EventSessionCompleted:
		out = &SessionCompletedPayload{}
	case EventSessionFailed:
		out = &SessionFailedPayload{}
	case EventModelStarted:
		out = &ModelStartedPayload{}
	case EventModelCompleted:
		out = &ModelCompletedPayload{}
	case EventTextDelta:
		out = &TextDeltaPayload{}
	case EventToolStarted:
		out = &ToolStartedPayload{}
	case EventToolFinished:
		out = &ToolFinishedPayload{}
	case EventArtifactCreated:
		out = &ArtifactCreatedPayload{}
	case EventUserMessage:
		out = &UserMessagePayload{}
	case EventUsage:
		out = &UsagePayload{}
	case EventResume:
		out = &ResumePayload{}
	case EventVerification:
		out = &VerificationPayload{}
	case EventInterruptedTool:
		out = &InterruptedToolPayload{}
	default:
		return raw
	}
	if err := json.Unmarshal(data, out); err != nil {
		return raw
	}
	return out
}

// splitLines splits raw bytes into lines, dropping a trailing empty line.
func splitLines(data []byte) [][]byte {
	var out [][]byte
	start := 0
	for i, b := range data {
		if b == '\n' {
			out = append(out, data[start:i])
			start = i + 1
		}
	}
	if start < len(data) {
		out = append(out, data[start:])
	}
	return out
}

// StoreArtifact writes an artifact's content.
func (f *FileStore) StoreArtifact(sessionID, artifactID string, content []byte) error {
	dir := filepath.Join(f.dir, "artifacts", sessionID)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, artifactID), content, 0o600)
}

// LoadArtifact reads up to limit bytes of an artifact starting at offset.
func (f *FileStore) LoadArtifact(sessionID, artifactID string, offset, limit int64) ([]byte, error) {
	fh, err := os.Open(f.artifactPath(sessionID, artifactID))
	if err != nil {
		return nil, err
	}
	defer fh.Close()
	if _, err := fh.Seek(offset, 0); err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 32 * 1024
	}
	buf := make([]byte, limit)
	n, err := fh.Read(buf)
	if err != nil && err.Error() != "EOF" {
		return nil, err
	}
	return buf[:n], nil
}

// ArtifactSize returns the stored size of an artifact.
func (f *FileStore) ArtifactSize(sessionID, artifactID string) (int64, error) {
	st, err := os.Stat(f.artifactPath(sessionID, artifactID))
	if err != nil {
		return 0, err
	}
	return st.Size(), nil
}

// ListSessions returns the session IDs known to the store.
func (f *FileStore) ListSessions() ([]string, error) {
	entries, err := os.ReadDir(filepath.Join(f.dir, "sessions"))
	if err != nil {
		return nil, err
	}
	var out []string
	for _, e := range entries {
		name := e.Name()
		if filepath.Ext(name) == ".jsonl" {
			out = append(out, name[:len(name)-len(".jsonl")])
		}
	}
	return out, nil
}

// leasePath returns the lease file for a session.
func (f *FileStore) leasePath(sessionID string) string {
	return filepath.Join(f.dir, "sessions", sessionID+".lease")
}

// AcquireLease takes the per-session lease. It refuses concurrent writers
// and recovers stale leases left by a crashed process.
func (f *FileStore) AcquireLease(sessionID string, ttl time.Duration) error {
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	path := f.leasePath(sessionID)
	now := time.Now().Unix()
	// Try to create the lease exclusively.
	fh, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err == nil {
		fmt.Fprintf(fh, "%d\n", now)
		fh.Close()
		return nil
	}
	if !os.IsExist(err) {
		return err
	}
	// The lease exists; check whether it is stale.
	data, rerr := os.ReadFile(path)
	if rerr != nil {
		return rerr
	}
	var stamp int64
	if _, serr := fmt.Sscanf(string(data), "%d", &stamp); serr != nil || time.Since(time.Unix(stamp, 0)) > ttl {
		// Stale: remove and retry once.
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
		fh, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err != nil {
			return fmt.Errorf("kite: session %s is leased by another writer", sessionID)
		}
		fmt.Fprintf(fh, "%d\n", now)
		fh.Close()
		return nil
	}
	return fmt.Errorf("kite: session %s is leased by another writer", sessionID)
}

// ReleaseLease releases the per-session lease.
func (f *FileStore) ReleaseLease(sessionID string) error {
	err := os.Remove(f.leasePath(sessionID))
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// HeartbeatLease refreshes the lease timestamp.
func (f *FileStore) HeartbeatLease(sessionID string) error {
	path := f.leasePath(sessionID)
	return os.WriteFile(path, []byte(fmt.Sprintf("%d\n", time.Now().Unix())), 0o600)
}
