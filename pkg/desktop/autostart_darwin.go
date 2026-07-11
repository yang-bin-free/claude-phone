//go:build darwin

package desktop

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
)

func AutostartPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Library", "LaunchAgents", LaunchAgentLabel+".plist"), nil
}

func InstallAutostart(executable string, args []string) error {
	path, err := AutostartPath()
	if err != nil {
		return err
	}
	b, err := launchAgentXML(executable, args)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	if err := os.WriteFile(path, b, 0o600); err != nil {
		return err
	}
	domain := "gui/" + strconv.Itoa(os.Getuid())
	_ = exec.Command("launchctl", "bootout", domain+"/"+LaunchAgentLabel).Run()
	return exec.Command("launchctl", "bootstrap", domain, path).Run()
}

func UninstallAutostart() error {
	path, err := AutostartPath()
	if err != nil {
		return err
	}
	domain := "gui/" + strconv.Itoa(os.Getuid())
	_ = exec.Command("launchctl", "bootout", domain+"/"+LaunchAgentLabel).Run()
	err = os.Remove(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func AutostartEnabled() bool {
	path, err := AutostartPath()
	if err != nil {
		return false
	}
	_, err = os.Stat(path)
	return err == nil
}
