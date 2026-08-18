//go:build !windows

package core

import (
	"os"
	"path/filepath"
)

// platformDataDir returns the XDG data directory for Kite on Unix systems.
func platformDataDir() string {
	if v := os.Getenv("XDG_DATA_HOME"); v != "" {
		return filepath.Join(v, "kite")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ".kite"
	}
	return filepath.Join(home, ".local", "share", "kite")
}
