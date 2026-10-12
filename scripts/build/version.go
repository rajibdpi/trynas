package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

var versionPattern = regexp.MustCompile(`^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-[A-Za-z0-9.-]+)?$`)

func nextVersion(tags []string) (string, error) {
	var highest [3]uint64
	found := false
	for _, tag := range tags {
		match := versionPattern.FindStringSubmatch(strings.TrimSpace(tag))
		if match == nil || match[4] != "" {
			continue
		}
		var parts [3]uint64
		for i := range parts {
			n, err := strconv.ParseUint(match[i+1], 10, 64)
			if err != nil {
				return "", fmt.Errorf("invalid version %q: %w", tag, err)
			}
			parts[i] = n
		}
		greater := !found
		for i := range parts {
			if parts[i] != highest[i] {
				greater = parts[i] > highest[i]
				break
			}
		}
		if greater {
			highest, found = parts, true
		}
	}
	if !found {
		return "v1.0.0", nil
	}
	if highest[2] == ^uint64(0) {
		return "", fmt.Errorf("patch version overflow")
	}
	return fmt.Sprintf("v%d.%d.%d", highest[0], highest[1], highest[2]+1), nil
}

func buildVersion(root, dist, explicit string) (string, error) {
	if explicit != "" {
		if !versionPattern.MatchString(explicit) {
			return "", fmt.Errorf("invalid version tag %q; use v1.0.0 or v1.0.0-rc.1", explicit)
		}
		return explicit, nil
	}
	cmd := exec.Command("git", "tag", "--list")
	cmd.Dir = root
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("read Git version tags (or provide --version): %w", err)
	}
	tags := strings.Fields(string(output))
	previous, err := os.ReadFile(filepath.Join(dist, "VERSION"))
	if err != nil && !os.IsNotExist(err) {
		return "", err
	}
	if err == nil {
		value := strings.TrimSpace(string(previous))
		if !versionPattern.MatchString(value) {
			return "", fmt.Errorf("invalid dist/VERSION %q", value)
		}
		tags = append(tags, value)
	}
	return nextVersion(tags)
}
