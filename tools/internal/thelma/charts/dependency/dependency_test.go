package dependency

import (
	"github.com/stretchr/testify/assert"
	"math/rand"
	"sort"
	"testing"
)

func TestWithDependents(t *testing.T) {
	g := testGraph(t)

	testCases := [][][]string{
		{
			{},
			{},
		},
		{
			{"d"},
			{"d"},
		},
		{
			{"f"},
			{"f"},
		},
		{
			{"b"},
			{"a", "b", "d"},
		},
		{
			{"b", "f"},
			{"a", "b", "d", "f"},
		},
		{
			{"c"},
			{"a", "b", "c", "d", "e"},
		},
	}

	for _, tc := range testCases {
		input, expected := tc[0], tc[1]
		actual := g.WithTransitiveDependents(input...)
		sort.Slice(actual, func(i, j int) bool {
			return actual[i] < actual[j]
		})
		assert.Equal(t, expected, actual, "input: %v", input)
	}
}

func TestTopoSort(t *testing.T) {
	g := testGraph(t)

	testCases := [][][]string{
		{
			{"a"},
			{"a"},
		},
		{
			{"a", "b"},
			{"b", "a"},
		},
		{
			{"a", "b", "c"},
			{"c", "b", "a"},
		},
		{
			{"a", "b", "c"},
			{"c", "b", "a"},
		},
		{
			{"d", "e"},
			{"e", "d"},
		},
		{
			{"c", "d", "e"},
			{"c", "e", "d"},
		},
		{
			{"b", "d"},
			{"b", "d"},
		},
	}

	for _, tc := range testCases {
		input, expected := tc[0], tc[1]
		rand.Shuffle(len(input), func(i, j int) {
			input[i], input[j] = input[j], input[i]
		})

		actual := make([]string, len(input))
		copy(actual, input)
		g.TopoSort(actual)
		assert.Equal(t, expected, actual, "input: %v", input)
	}
}

func TestCycleDetection(t *testing.T) {
	_, err := NewGraph(map[string][]string{
		"a": {"a"},
	})
	assert.Error(t, err)
	assert.Regexp(t, "cycle detected: a -> a", err.Error())

	_, err = NewGraph(map[string][]string{
		"a": {"b"},
		"b": {"a"},
	})
	assert.Error(t, err)
	assert.Regexp(t, "cycle detected", err.Error())
	assert.Regexp(t, "a -> b", err.Error())
	assert.Regexp(t, "b -> a", err.Error())

	_, err = NewGraph(map[string][]string{
		"a": {"b"},
		"b": {"c"},
		"c": {"d"},
		"d": {"e"},
		"e": {"c"},
	})
	assert.Error(t, err)
	assert.Regexp(t, "cycle detected", err.Error())
	assert.Regexp(t, "c -> d", err.Error())
	assert.Regexp(t, "d -> e", err.Error())
	assert.Regexp(t, "e -> c", err.Error())
}

func testGraph(t *testing.T) *Graph {
	deps := map[string][]string{
		"a": {"b", "c"},
		"b": {"c"},
		"c": {},
		"d": {"e", "a"},
		"e": {"c"},
		"f": {},
	}
	g, err := NewGraph(deps)
	if err != nil {
		t.Fatal(err)
	}
	return g
}
