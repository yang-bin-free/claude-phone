package product

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// ResolveDataDir returns the configured data directory and migrates the legacy
// default directory only when no explicit directory or new default exists.
func ResolveDataDir(home, explicit string) (path string, migrated bool, err error) {
	if explicit != "" {
		return explicit, false, nil
	}
	current := DefaultDataDir(home)
	legacy := filepath.Join(home, LegacyDataDirName)
	if _, err := os.Stat(current); err == nil {
		return current, false, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", false, err
	}
	if _, err := os.Stat(legacy); errors.Is(err, os.ErrNotExist) {
		return current, false, nil
	} else if err != nil {
		return "", false, err
	}
	if err := os.Rename(legacy, current); err != nil {
		return "", false, fmt.Errorf("migrate CodeAfar data: %w", err)
	}
	return current, true, nil
}
