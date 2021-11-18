package semver

import (
	"fmt"
	"golang.org/x/mod/semver"
	"strconv"
	"strings"
)

// This is a wrapper around Go mod's semver with some additional logic

// IsValid returns true if this is a valid semantic version
func IsValid(version string) bool {
	return semver.IsValid(normalize(version))
}

// Compare compares two semantic versions
// Returns 0 if v == w, -1 if v < w, or +1 if v > w.
func Compare(v string, w string) int {
	return semver.Compare(normalize(v), normalize(w))
}

// MinorBump bumps the minor version of a semantic version. Eg.
// MinorBump("1.2.3") -> "1.3.0"
func MinorBump(version string) (string, error) {
	if !IsValid(version) {
		return "", fmt.Errorf("invalid semantic version %q", version)
	}

	tokens := strings.SplitN(version, ".", 3)
	if len(tokens) < 2 {
		return "", fmt.Errorf("invalid semantic version %q", version)
	}
	major := tokens[0]
	minor, err := strconv.Atoi(tokens[1])
	if err != nil {
		return "", fmt.Errorf("invalid semantic version %q: %v", version, err)
	}

	return fmt.Sprintf("%s.%d.0", major, minor+1), nil
}

// go mod's semver implementation expects versions to be prefixed with "v"
// This function removes the prefix.
func normalize(chartVersion string) string {
	if !strings.HasPrefix(chartVersion, "v") {
		chartVersion = fmt.Sprintf("v%s", chartVersion)
	}
	return chartVersion
}
