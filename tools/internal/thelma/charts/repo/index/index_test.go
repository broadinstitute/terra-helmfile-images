package index

import (
	"github.com/stretchr/testify/assert"
	"os"
	"path"
	"testing"
)

func TestIndex_MostRecentVersion(t *testing.T) {
	index, err := LoadFromFile("testdata/index.yaml")
	assert.NoError(t, err)

	assert.Equal(t, "0.19.0", index.MostRecentVersion("agora"))
	assert.Equal(t, "0.4.0", index.MostRecentVersion("sherlock-reporter"))
	assert.Equal(t, "", index.MostRecentVersion("unknown-chart"))

	emptyFile := path.Join(t.TempDir(), "empty.yaml")
	_, err = os.Create(emptyFile)
	assert.NoError(t, err)

	emptyIndex, err := LoadFromFile(emptyFile)
	assert.NoError(t, err)
	assert.Equal(t, "", emptyIndex.MostRecentVersion("agora"))
}
