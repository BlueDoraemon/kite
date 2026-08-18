//go:build windows

package core

import (
	"os"
	"path/filepath"
)

// platformDataDir returns the LOCALAPPDATA data directory for Kite on Windows.
func platformDataDir() string {
	if v := os.Getenv("LOCALAPPDATA"); v != "" {
		return filepath.Join(v, "kite")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ".kite"
	}
	return filepath.Join(home, "AppData", "Local", "kite")
}
