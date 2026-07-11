//go:build !darwin

package desktop

import "errors"

func AutostartPath() (string, error) { return "", errors.New("autostart is only supported on macOS") }
func InstallAutostart(string, []string) error {
	return errors.New("autostart is only supported on macOS")
}
func UninstallAutostart() error { return errors.New("autostart is only supported on macOS") }
func AutostartEnabled() bool    { return false }
