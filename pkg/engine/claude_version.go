package engine

import (
	"errors"
	"os/exec"
	"regexp"
)

var cliVersionPattern = regexp.MustCompile(`\b(\d+\.\d+\.\d+(?:[-+][0-9A-Za-z.-]+)?)\b`)
var claudeVersionPattern = cliVersionPattern

func DetectClaudeVersion(bin string) (string, error) {
	if bin == "" {
		bin = "claude"
	}
	return DetectCLIVersion(bin, "Claude")
}

func DetectCLIVersion(bin, productName string) (string, error) {
	output, err := exec.Command(bin, "--version").CombinedOutput()
	if err != nil {
		return "", err
	}
	match := cliVersionPattern.FindSubmatch(output)
	if len(match) != 2 {
		return "", errors.New("unable to parse " + productName + " CLI version")
	}
	return string(match[1]), nil
}
