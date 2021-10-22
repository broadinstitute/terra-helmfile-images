package matchers

import (
	"github.com/stretchr/testify/assert"
	"regexp"
	"strings"
	"testing"
)

func TestStringMatchers(t *testing.T) {
	type testCase struct {
		name           string
		matcher        StringMatcher
		expectedString string
		matches        map[string]bool
	}

	testCases := []testCase{{
		name:           "equalsFoo",
		matcher:        Equals("foo"),
		expectedString: "foo",
		matches: map[string]bool{
			"foo": true,
			"bar": false,
		},
	}, {
		name:           "startsWithFRegexp",
		matcher:        MatchesRegexp(regexp.MustCompile("^f")),
		expectedString: "match(/^f/)",
		matches: map[string]bool{
			"foo": true,
			"bar": false,
		},
	}, {
		name: "endsWithOPredicate",
		matcher: MatchesPredicate("ends with o", func(s string) bool {
			return strings.HasSuffix(s, "o")
		}),
		expectedString: "<ends with o>",
		matches: map[string]bool{
			"foo": true,
			"bar": false,
		},
	}, {
		name:           "containsOO",
		matcher:        Contains("oo"),
		expectedString: "contains(oo)",
		matches: map[string]bool{
			"foo": true,
			"bar": false,
		},
	}, {
		name:           "any",
		matcher:        AnyString(),
		expectedString: "<any>",
		matches: map[string]bool{
			"foo": true,
			"bar": true,
		},
	}, {
		name:           "none",
		matcher:        None(),
		expectedString: "<none>",
		matches: map[string]bool{
			"foo": false,
			"bar": false,
		},
	}, {
		name:           "not",
		matcher:        Not(Equals("foo")),
		expectedString: "not(foo)",
		matches: map[string]bool{
			"foo": false,
			"bar": true,
		},
	}}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expectedString, tc.matcher.String())
			for input, expected := range tc.matches {
				assert.Equal(t, expected, tc.matcher.Matches(input), "matcher should return %v for input %q", expected, input)
			}
		})
	}
}
