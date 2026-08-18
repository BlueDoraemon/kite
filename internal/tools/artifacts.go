package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Artifact limits. Outputs larger than maxInline size are stored to disk and
// replaced in the tool result by a short reference the model can follow up on
// with another read call.
const (
	// maxInline is the largest output kept inline in a tool result.
	maxInline = 16 * 1024
	// maxArtifactBytes is the largest output stored as an artifact; larger
	// outputs are still referenced but the artifact is truncated.
	maxArtifactBytes = 2 * 1024 * 1024
)

// truncMark is appended to truncated content so consumers know it is not the
// full artifact.
const truncMark = "\n... output truncated ...\n"

// artifactDir returns the reference into the store. Outputs are kept on disk
// under the repository's .kite/artifacts directory, keyed by tool name and a
// timestamp, so the model can read them back with the read tool.
func (s *Set) artifactDir() string {
	return filepath.Join(s.Dir, ".kite", "artifacts")
}

// storeArtifact writes content to the artifact store and returns a result
// string that references the artifact path. The returned reference includes a
// preview so the model can tell whether it even needs to read the file.
func (s *Set) storeArtifact(tool, content string) string {
	dir := s.artifactDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		// The store must never be the cause of a failed tool call; fall
		// back to the inline truncated output.
		return truncateForModel(content)
	}
	name := fmt.Sprintf("%s-%d.txt", tool, time.Now().UnixNano())
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return truncateForModel(content)
	}
	rel, err := filepath.Rel(s.Dir, path)
	if err != nil {
		rel = path
	}
	originalSize := len(content)
	if originalSize > maxArtifactBytes {
		content = content[:maxArtifactBytes] + truncMark
	}
	preview := truncateForModel(content)
	return fmt.Sprintf(
		"Output is %d bytes; stored as artifact at %s (read it with the read tool).\nPreview:\n%s",
		originalSize, rel, preview)
}

// truncateForModel keeps the head and tail of content so a truncated preview
// still gives the model a sense of the whole output.
func truncateForModel(content string) string {
	if len(content) <= maxInline {
		return content
	}
	head := content[:maxInline/2]
	tail := content[len(content)-maxInline/2:]
	return fmt.Sprintf("%s\n... (truncated; %d bytes total) ...\n%s", head, len(content), tail)
}

// inlineIfSmall returns content unchanged if it fits inline, otherwise stores
// it as an artifact and returns the reference.
func (s *Set) inlineIfSmall(tool, content string) string {
	if len(content) <= maxInline {
		return content
	}
	return s.storeArtifact(tool, content)
}
