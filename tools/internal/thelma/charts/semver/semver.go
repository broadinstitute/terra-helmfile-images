package semver

import (
	"fmt"
	"golang.org/x/mod/semver"
	"strconv"
	"strings"
)

func IsValid(version string) bool {
	return semver.IsValid(normalize(version))
}

func Compare(v string, w string) int {
	return semver.Compare(normalize(v), normalize(w))
}

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

func normalize(chartVersion string) string {
	if !strings.HasPrefix(chartVersion, "v") {
		// Gomod's semver implementation expects versions to be prefixed with "v"
		chartVersion = fmt.Sprintf("v%s", chartVersion)
	}
	return chartVersion
}