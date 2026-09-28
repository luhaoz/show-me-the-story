package desktop

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// SaveResult is returned to the frontend. Cancelled is not an error and must
// not produce a success notification.
type SaveResult struct {
	Cancelled bool   `json:"cancelled"`
	Path      string `json:"path,omitempty"`
}

// SafeSuggestedName strips directory components and characters that Windows
// does not permit in a file name.
func SafeSuggestedName(name, fallback string) string {
	name = filepath.Base(strings.TrimSpace(name))
	if name == "." || name == "" {
		name = fallback
	}
	name = strings.Map(func(r rune) rune {
		if r < 32 || strings.ContainsRune(`<>:"/\|?*`, r) {
			return '_'
		}
		return r
	}, name)
	name = strings.TrimRight(name, ". ")
	if name == "" {
		return fallback
	}
	return name
}

// WriteReaderAtomic streams content into the destination directory and then
// replaces the destination with a rollback rename. Existing files survive
// every failure before the final commit.
func WriteReaderAtomic(path string, src io.Reader) (err error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("resolve save path: %w", err)
	}
	dir := filepath.Dir(abs)
	if info, statErr := os.Stat(dir); statErr != nil || !info.IsDir() {
		return fmt.Errorf("save directory unavailable: %w", statErr)
	}
	tmp, err := os.CreateTemp(dir, ".show-me-the-story-save-*")
	if err != nil {
		return fmt.Errorf("create temporary save file: %w", err)
	}
	tmpPath := tmp.Name()
	defer func() {
		tmp.Close()
		os.Remove(tmpPath)
	}()
	if _, err = io.Copy(tmp, src); err != nil {
		return fmt.Errorf("write temporary save file: %w", err)
	}
	if err = tmp.Sync(); err != nil {
		return fmt.Errorf("sync temporary save file: %w", err)
	}
	if err = tmp.Close(); err != nil {
		return fmt.Errorf("close temporary save file: %w", err)
	}

	rollback := ""
	if _, statErr := os.Stat(abs); statErr == nil {
		backup, createErr := os.CreateTemp(dir, ".show-me-the-story-rollback-*")
		if createErr != nil {
			return fmt.Errorf("reserve rollback file: %w", createErr)
		}
		rollback = backup.Name()
		if closeErr := backup.Close(); closeErr != nil {
			return fmt.Errorf("close rollback reservation: %w", closeErr)
		}
		if removeErr := os.Remove(rollback); removeErr != nil {
			return fmt.Errorf("prepare rollback path: %w", removeErr)
		}
		if renameErr := os.Rename(abs, rollback); renameErr != nil {
			return fmt.Errorf("protect existing file: %w", renameErr)
		}
	} else if !os.IsNotExist(statErr) {
		return fmt.Errorf("inspect destination: %w", statErr)
	}

	if err = os.Rename(tmpPath, abs); err != nil {
		if rollback != "" {
			if restoreErr := os.Rename(rollback, abs); restoreErr != nil {
				return fmt.Errorf("commit save: %v; restore existing file: %w", err, restoreErr)
			}
		}
		return fmt.Errorf("commit save: %w", err)
	}
	if rollback != "" {
		if err = os.Remove(rollback); err != nil {
			return fmt.Errorf("save committed but rollback cleanup failed: %w", err)
		}
	}
	return nil
}
