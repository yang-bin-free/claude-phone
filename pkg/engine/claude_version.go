package engine

import (
	"errors"
	"os/exec"
	"regexp"
)

var claudeVersionPattern = regexp.MustCompile(`\b(\d+\.\d+\.\d+(?:[-+][0-9A-Za-z.-]+)?)\b`)

func DetectClaudeVersion(bin string) (string, error) {
	if bin == "" {
		bin = "claude"
	}
	output, err := exec.Command(bin, "--version").CombinedOutput()
	if err != nil {
		return "", err
	}
	match := claudeVersionPattern.FindSubmatch(output)
	if len(match) != 2 {
		return "", errors.New("unable to parse Claude CLI version")
	}
	return string(match[1]), nil
}
