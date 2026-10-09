package main

import (
	"fmt"
	"runtime"
	"strings"
)

var supported = []string{"linux/amd64", "linux/arm64", "windows/amd64", "windows/arm64", "darwin/amd64", "darwin/arm64"}

func targets(all bool, value string) ([]string, error) {
	if all && value != "" {
		return nil, fmt.Errorf("use either --all or --target")
	}
	if all {
		return append([]string(nil), supported...), nil
	}
	if value == "" {
		value = runtime.GOOS + "/" + runtime.GOARCH
	}
	seen := map[string]bool{}
	result := []string{}
	for _, target := range strings.Split(value, ",") {
		target = strings.TrimSpace(target)
		valid := false
		for _, candidate := range supported {
			valid = valid || target == candidate
		}
		if !valid {
			return nil, fmt.Errorf("unsupported target %q; supported: %s", target, strings.Join(supported, ", "))
		}
		if !seen[target] {
			result = append(result, target)
			seen[target] = true
		}
	}
	return result, nil
}

func artifactName(target string) string {
	name := "trynas-" + strings.ReplaceAll(target, "/", "-")
	if strings.HasPrefix(target, "windows/") {
		name += ".exe"
	}
	return name
}
