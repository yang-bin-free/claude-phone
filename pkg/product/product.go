// Package product centralizes CodeAfar's user-visible identity and local paths.
package product

import "path/filepath"

const (
	Name              = "CodeAfar"
	Tagline           = "Run locally. Code from anywhere."
	DataDirName       = ".codeafar"
	LegacyDataDirName = ".claude-phone"
)

func DefaultDataDir(home string) string {
	if home == "" {
		return DataDirName
	}
	return filepath.Join(home, DataDirName)
}
