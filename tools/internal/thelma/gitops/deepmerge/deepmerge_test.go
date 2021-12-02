package deepmerge

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

type fakeStruct struct {
	StringVal         string `yaml:"stringVal"`
	OnlyInFile1String string `yaml:"onlyInFile1String"`
	OnlyInFile2Bool   bool   `yaml:"onlyInFile2Bool"`
	OnlyInFile3Int    int    `yaml:"onlyInFile3Int"`

	Nested Nested `yaml:"nested"`

	Map map[string]string `yaml:"map"`
}

type Nested struct {
	LevelOne LevelOne `yaml:"levelOne"`
}

type LevelOne struct {
	LevelTwo LevelTwo `yaml:"levelTwo"`
}

type LevelTwo struct {
	KeyA string `yaml:"keyA"`
	KeyB string `yaml:"keyB"`
	KeyC string `yaml:"keyC"`
	KeyD string `yaml:"keyD"`
	KeyE string `yaml:"keyE"`
	KeyF string `yaml:"keyF"`
}

func TestUnmarshal(t *testing.T) {
	var result fakeStruct

	err := Unmarshal(&result, "testdata/file1.yaml", "testdata/missing.yaml", "testdata/file2.yaml", "testdata/file3.yaml", "testdata/empty.yaml")
	if !assert.NoError(t, err) {
		t.FailNow()
	}

	assert.Equal(t, "file3", result.StringVal)
	assert.Equal(t, "file1", result.OnlyInFile1String)
	assert.Equal(t, true, result.OnlyInFile2Bool)
	assert.Equal(t, 100, result.OnlyInFile3Int)

	m := result.Map
	assert.Equal(t, "file1", m["keyA"])
	assert.Equal(t, "file2", m["keyB"])
	assert.Equal(t, "file3", m["keyC"])

	l2 := result.Nested.LevelOne.LevelTwo
	assert.Equal(t, "file3", l2.KeyA)
	assert.Equal(t, "file2", l2.KeyB)
	assert.Equal(t, "file1", l2.KeyC)
	assert.Equal(t, "file2", l2.KeyD)
	assert.Equal(t, "file3", l2.KeyE)
	assert.Equal(t, "file3", l2.KeyF)
}
