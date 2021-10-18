package helmfile

import (
	"github.com/stretchr/testify/assert"
	"os"
	"path"
	"testing"
)

// Integration test does not exercise normalizeRenderDirectories(), so add a unit test here
func TestNormalizeOutputDir(t *testing.T) {
	// Create tmpdir
	outputDir := t.TempDir()

	// Create some fake helmfile output directories
	manifestDirs := []string{
		"helmfile-b47efc70-leonardo",
		"helmfile-a14e02c1-cromwell",
		"this-should-not-match",
	}

	for _, manifestDir := range manifestDirs {
		if err := os.MkdirAll(path.Join(outputDir, manifestDir), 0755); err != nil {
			t.Error(err)
			return
		}
	}

	err := normalizeOutputDir(outputDir)
	if !assert.NoError(t, err) {
		return
	}

	for _, dir := range []string{"leonardo", "cromwell", "this-should-not-match"} {
		assert.DirExists(t, path.Join(outputDir, dir))
	}
	assert.NoDirExists(t, path.Join(outputDir, manifestDirs[0]))
	assert.NoDirExists(t, path.Join(outputDir, manifestDirs[1]))
}
