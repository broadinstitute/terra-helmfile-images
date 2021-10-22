package matchers

import (
	"fmt"
	"regexp"
	"strings"
)

// StringMatcher is used for flexible matching of command attributes.
type StringMatcher struct {
	predicate func(string) bool
	stringer  func() string
}

// Matches returns true if the the given string matches this matcher, false otherwise
func (sm StringMatcher) Matches(actual string) bool {
	return sm.predicate(actual)
}

// String returns a string representation of this matcher, useful for debugging
func (sm StringMatcher) String() string {
	return sm.stringer()
}

// Equals will check if a string is equal to given argument
func Equals(expected string) StringMatcher {
	return StringMatcher{
		predicate: func(actual string) bool {
			return actual == expected
		},
		stringer: func() string {
			return expected
		},
	}
}

// MatchesRegexp will check if a string matches the given regexp
func MatchesRegexp(expected *regexp.Regexp) StringMatcher {
	return StringMatcher{
		predicate: func(actual string) bool {
			return expected.MatchString(actual)
		},
		stringer: func() string {
			return fmt.Sprintf("match(/%s/)", expected.String())
		},
	}
}

// MatchesPredicate will check if a string matches the given predicate
func MatchesPredicate(description string, predicate func(string) bool) StringMatcher {
	return StringMatcher{
		predicate: predicate,
		stringer: func() string {
			return fmt.Sprintf("<%s>", description)
		},
	}
}

// Contains will check if a string contains the given substring
func Contains(expected string) StringMatcher {
	return StringMatcher{
		predicate: func(actual string) bool {
			return strings.Contains(actual, expected)
		},
		stringer: func() string {
			return fmt.Sprintf("contains(%s)", expected)
		},
	}
}

// AnyString will match any string
func AnyString() StringMatcher {
	return StringMatcher{
		predicate: func(string) bool {
			return true
		},
		stringer: func() string {
			return "<any>"
		},
	}
}

// None will return a matcher that never matches any string.
func None() StringMatcher {
	return StringMatcher{
		predicate: func(string) bool {
			return false
		},
		stringer: func() string {
			return "<none>"
		},
	}
}

// Not will invert a string matcher
func Not(m StringMatcher) StringMatcher {
	return StringMatcher{
		predicate: func(actual string) bool {
			return !m.predicate(actual)
		},
		stringer: func() string {
			return fmt.Sprintf("not(%s)", m.stringer())
		},
	}
}

// toStringMatcher converts a generic argument to a string matcher
func toStringMatcher(expected interface{}) StringMatcher {
	switch typed := expected.(type) {
	case string:
		return Equals(typed)
	case *regexp.Regexp:
		return MatchesRegexp(typed)
	case StringMatcher:
		return typed
	default:
		panic(fmt.Errorf("expected string or StringMatcher, got %T", typed))
	}
}
