package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

// maxArtifactRead caps how much of an artifact is returned per retrieval.
const maxArtifactRead = 32 * 1024

// Artifact returns a Tool that retrieves a stored artifact by ID and byte
// offset. It reads from the Kite data directory.
func (s *Set) Artifact() *Tool {
	return &Tool{
		name:        "artifact",
		description: "Retrieve a stored artifact by ID, optionally from a byte offset. Returns up to 32 KiB.",
		specs: []argSpec{
			{name: "id", typ: "string", desc: "The artifact ID to retrieve.", required: true},
			{name: "offset", typ: "integer", desc: "Byte offset to start reading from."},
			{name: "limit", typ: "integer", desc: "Maximum bytes to read."},
		},
		run: func(ctx context.Context, args map[string]any) (string, error) {
			id := str(args, "id")
			offset := intv(args, "offset")
			limit := intv(args, "limit")
			if id == "" {
				return "", fmt.Errorf("id must not be empty")
			}
			if limit <= 0 || limit > maxArtifactRead {
				limit = maxArtifactRead
			}
			data, err := s.readArtifact(id, int64(offset), int64(limit))
			if err != nil {
				return "", err
			}
			return string(data), nil
		},
	}
}

// readArtifact reads an artifact from the data directory. The artifact ID
// encodes the session prefix (art_<session>_<id>), so no session argument is
// needed.
func (s *Set) readArtifact(id string, offset, limit int64) ([]byte, error) {
	// The artifact ID format is art_<sessionID>_<rand>. Find the file by
	// scanning the artifacts directory.
	dataDir := os.Getenv("KITE_DATA_DIR")
	if dataDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		dataDir = filepath.Join(home, ".local", "share", "kite")
	}
	root := filepath.Join(dataDir, "artifacts")
	// Search all session directories for the artifact file.
	var found string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() && info.Name() == id {
			found = path
			return filepath.SkipAll
		}
		return nil
	})
	if err != nil || found == "" {
		return nil, fmt.Errorf("artifact %s not found", id)
	}
	fh, err := os.Open(found)
	if err != nil {
		return nil, err
	}
	defer fh.Close()
	if _, err := fh.Seek(offset, 0); err != nil {
		return nil, err
	}
	buf := make([]byte, limit)
	n, err := fh.Read(buf)
	if err != nil && err.Error() != "EOF" {
		return nil, err
	}
	return buf[:n], nil
}
