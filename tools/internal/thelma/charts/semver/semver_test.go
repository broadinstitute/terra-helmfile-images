package semver

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestIsValid(t *testing.T) {
	testCases := map[string]bool{
		"":           false,
		"invalid":    false,
		"0":          true,
		"1.2.3":      true,
		"1.2.3-beta": true,
	}
	for input, expected := range testCases {
		assert.Equal(t, expected, IsValid(input), "input: %v", input)
	}
}

func TestCompare(t *testing.T) {
	pairs := [][]string{
		{"0", "1"},
		{"0.2.4", "1.2.4"},
		{"0.2.4", "0.3.4"},
		{"0.2.4", "0.2.5"},
		{"0.2.4-beta", "0.2.4"},
	}

	for _, pair := range pairs {
		v1, v2 := pair[0], pair[1]
		assert.Equal(t, 0, Compare(v1, v1), "%v == %v", v1, v1)
		assert.Equal(t, 0, Compare(v2, v2), "%v == %v", v2, v2)
		assert.Equal(t, -1, Compare(v1, v2), "%v < %v", v1, v2)
		assert.Equal(t, 1, Compare(v2, v1), "%v > %v", v2, v1)
	}
}

func TestMinorBump(t *testing.T) {
	failCases := []string{
		"",
		"invalid",
		"1",
	}
	successCases := map[string]string{
		"1.2":        "1.3.0",
		"1.2.3":      "1.3.0",
		"1.2.3-beta": "1.3.0",
	}

	for _, input := range failCases {
		_, err := MinorBump(input)
		assert.Error(t, err, "input: %v", input)
	}

	for input, expected := range successCases {
		bumped, err := MinorBump(input)
		assert.NoError(t, err, "input: %v", input)
		assert.Equal(t, expected, bumped, "input: %v", input)
	}
}
