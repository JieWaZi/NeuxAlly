package runtimes

import (
	"github.com/JieWaZi/NeuxAlly/server/constants/status_error"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

func resolveExecutable(
	candidates []string,
	environment environmentSnapshot,
) (string, error) {
	var failures []string
	for _, candidate := range candidates {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		resolved, err := resolveExecutableCandidate(candidate, environment)
		if err == nil {
			return resolved, nil
		}
		failures = append(failures, err.Error())
	}
	if len(failures) == 0 {
		return "", status_error.RuntimeUnavailable
	}
	return "", status_error.RuntimeUnavailable
}

func resolveExecutableCandidate(
	candidate string,
	environment environmentSnapshot,
) (string, error) {
	if filepath.IsAbs(candidate) || strings.ContainsAny(candidate, `/\`) {
		return validateExecutablePath(candidate)
	}
	pathValue := environment.get("PATH")
	for _, directory := range filepath.SplitList(pathValue) {
		if directory == "" {
			continue
		}
		if runtime.GOOS == "windows" {
			for _, extension := range windowsExecutableExtensions(candidate, environment) {
				path := filepath.Join(directory, candidate+extension)
				if resolved, err := validateExecutablePath(path); err == nil {
					return resolved, nil
				}
			}
			continue
		}
		path := filepath.Join(directory, candidate)
		if resolved, err := validateExecutablePath(path); err == nil {
			return resolved, nil
		}
	}
	return "", status_error.BadRequest
}

func windowsExecutableExtensions(
	candidate string,
	environment environmentSnapshot,
) []string {
	if filepath.Ext(candidate) != "" {
		return []string{""}
	}
	pathExt := environment.get("PATHEXT")
	if pathExt == "" {
		pathExt = ".COM;.EXE;.BAT;.CMD"
	}
	parts := strings.Split(pathExt, ";")
	extensions := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			extensions = append(extensions, part)
		}
	}
	return extensions
}

func validateExecutablePath(path string) (string, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", status_error.RuntimeUnavailable
	}
	info, err := os.Stat(absolute)
	if err != nil {
		return "", status_error.RuntimeUnavailable
	}
	if info.IsDir() {
		return "", status_error.RuntimeUnavailable
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0o111 == 0 {
		return "", status_error.RuntimeUnavailable
	}
	return absolute, nil
}
