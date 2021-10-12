package matchers

import (
	"fmt"
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/shell"
	"strings"
)

type envConstraint struct {
	name         string        // name of the environment variable this constraint matches
	valueMatcher StringMatcher // valueMatcher matches the value of the environment variable
}

// CmdMatcher is for evaluating whether a shell.Command matches configured
// constraints.
type CmdMatcher struct {
	progConstraint StringMatcher
	argConstraints []StringMatcher
	envConstraints []envConstraint
	dirConstraint  StringMatcher
	// TODO check pristineEnv? (haven't encountered a use case where it was necessary to test)
	allowExtraEnvVars bool
	allowExtraArgs    bool
}

// newCmdMatcher initializes a new command matchers with sensible defaults
func newCmdMatcher() *CmdMatcher {
	matcher := new(CmdMatcher)
	matcher.progConstraint = Equals("")
	matcher.dirConstraint = AnyString()
	return matcher
}

// AnyCmd returns a new command matcher that will match any command
func AnyCmd() *CmdMatcher {
	matcher := newCmdMatcher()
	matcher.WithProg(AnyString())
	matcher.AllowExtraEnvVars()
	matcher.AllowExtraArgs()
	return matcher
}

// CmdWithArgs returns a new command matcher that will the given prog + args.
//
// Eg. CmdWithArgs("ls", "-al", "/tmp")
// will match
// shell.Command{
//   Prog: "ls",
//   Args: []string{"-al", "/tmp"}
// }
func CmdWithArgs(prog interface{}, args ...interface{}) *CmdMatcher {
	matcher := newCmdMatcher()
	matcher.WithProg(prog)
	matcher.WithExactArgs(args...)
	matcher.FailIfExtraArgs()
	matcher.FailIfExtraEnvVars()
	return matcher
}

// CmdWithEnv returns a new command matcher that will match the given prog, arg, and any
// leading NAME=VAL environment variable pairs
//
// Eg. CmdWithEnv("FOO=BAR", "HOME=/tmp", "ls", "-al", "~")
// will match
// shell.Command{
//   Env: []string{"FOO=BAR", "HOME=/tmp"},
//   Prog: "ls",
//   Args: []string{"-al", "~"},
// }
func CmdWithEnv(args ...string) *CmdMatcher {
	// count number of leading NAME=VALUE environment var pairs preceding command
	var i int
	for i = 0; i < len(args); i++ {
		if !strings.Contains(args[i], "=") {
			// if this is not a NAME=VALUE pair, exit
			break
		}
	}

	numEnvVars := i
	progIndex := i
	numArgs := len(args) - (numEnvVars + 1)

	matcher := newCmdMatcher()

	if numEnvVars > 0 {
		matcher.WithExactEnvVars(args[0:numEnvVars]...)
	}
	if progIndex < len(args) {
		matcher.WithProg(args[progIndex])
	}
	if numArgs > 0 {
		cmdArgs := args[progIndex+1:]
		matcher.WithExactArgs(toGeneric(cmdArgs)...)
	}

	return matcher
}

// CmdFromString returns a new command matcher that will match the given command, supplied as a string
// and parsed
//
// Eg. CmdFromString("FOO=BAR HOME=/tmp ls -al ~")
// will match
// shell.Command{
//   Env: []string{"FOO=BAR", "HOME=/tmp"},
//   Prog: "ls",
//   Args: []string{"-al", "~"},
// }
func CmdFromString(cmd string) *CmdMatcher {
	return CmdWithEnv(strings.Fields(cmd)...)
}

// CmdFromFmt returns a new command matcher that will match the given command, supplied as a format
// string with arguments.
//
// Eg. CmdFromFmt("FOO=BAR HOME=%s ls -al %s", "/root", "/")
// will match
// shell.Command{
//   Env: []string{"FOO=BAR", "HOME=/root"},
//   Prog: "ls",
//   Args: []string{"-al", "/"},
// }
func CmdFromFmt(cmdFmt string, args... interface{}) *CmdMatcher {
	formatted := fmt.Sprintf(cmdFmt, args...)
	return CmdFromString(formatted)
}

// WithProg configures the matcher to expect a prog matching
// the given argument matcher
//
// eg. matcher.WithProg("ls")
func (m *CmdMatcher) WithProg(matcher interface{}) *CmdMatcher {
	m.progConstraint = toStringMatcher(matcher)
	return m
}

// WithExactArgs configures the matcher to expect the exact given
// set of argument matchers
//
// eg. matcher.WithExactArgs("-al", "/tmp")
//     matcher.WithExactArgs("-al", Contains("/tmp"))
func (m *CmdMatcher) WithExactArgs(matchers ...interface{}) *CmdMatcher {
	args := make([]StringMatcher, len(matchers))
	for i, matcher := range matchers {
		args[i] = toStringMatcher(matcher)
	}
	m.argConstraints = args
	m.FailIfExtraArgs()
	return m
}

// WithArgAt configures the matcher to expect an argument matching the given matcher.
//
// Subseq
//
// eg. matcher.WithArgAt(0, AnyString()).
//       WithArgAt(1, "exact string").
//       WithArgAt(2, MatchesRegexp(Regexp.MustCompile("foo.*")))
func (m *CmdMatcher) WithArgAt(index int, matcher interface{}) *CmdMatcher {
	// Pad argconstraints up to index with AnyString() matchers
	for len(m.argConstraints) < index {
		m.argConstraints = append(m.argConstraints, AnyString())
	}

	sm := toStringMatcher(matcher)
	if len(m.argConstraints) > index {
		m.argConstraints[index] = sm
	} else {
		m.argConstraints = append(m.argConstraints, sm)
	}

	return m
}

// WithExactEnvVars configures the matcher to expect the exact given
// set of env vars, supplied as NAME=VALUE pairs.
//
// eg. matcher.WithExactEnvVars("FOO=BAR", "BAZ=QUUX")
func (m *CmdMatcher) WithExactEnvVars(pairs ...string) *CmdMatcher {
	envConstraints := make([]envConstraint, len(pairs))
	for i, pair := range pairs {
		name, value := splitEnvPair(pair)
		constraint := envConstraint{
			name:         name,
			valueMatcher: Equals(value),
		}
		envConstraints[i] = constraint
	}
	m.envConstraints = envConstraints
	m.FailIfExtraEnvVars()
	return m
}

// WithEnvVar configures the matcher to expect an env var matching the given matcher.
// May be called multiple times to match different env variable names.
//
// eg. matcher.WithEnvVar("FOO", "BAR").WithEnvVar("BAZ", HasPrefix("FOO_"))
func (m *CmdMatcher) WithEnvVar(name string, matcher interface{}) *CmdMatcher {
	constraint := envConstraint{
		name:         name,
		valueMatcher: toStringMatcher(matcher),
	}
	m.envConstraints = append(m.envConstraints, constraint)
	return m
}

// WithDir configures the matcher to exppect a directory matching the given matcher
func (m *CmdMatcher) WithDir(matcher interface{}) *CmdMatcher {
	m.dirConstraint = toStringMatcher(matcher)
	return m
}

// Matches returns true if the given command matches
func (m *CmdMatcher) Matches(cmd shell.Command) bool {
	// check Prog
	if !m.progConstraint.Matches(cmd.Prog) {
		return false
	}

	// checkArgs
	if !m.matchesArgs(cmd) {
		return false
	}

	// check Env
	if !m.matchesEnvVars(cmd) {
		return false
	}

	// check Dir
	if !m.dirConstraint.Matches(cmd.Dir) {
		return false
	}

	return true
}

// FailIfExtraEnvVars configures the matcher to fail the match if there are any
// extra env vars in a given shell.Command that don't match the given matchers
func (m *CmdMatcher) FailIfExtraEnvVars() *CmdMatcher {
	m.allowExtraEnvVars = false
	return m
}

// AllowExtraEnvVars configures the matcher to ignore any
// extra env vars in a given shell.Command that don't match the given matchers
func (m *CmdMatcher) AllowExtraEnvVars() *CmdMatcher {
	m.allowExtraEnvVars = true
	return m
}

// FailIfExtraArgs configures the matcher to fail the match if there are any
// extra arguments in a given shell.Command that don't match the given matchers
func (m *CmdMatcher) FailIfExtraArgs() *CmdMatcher {
	m.allowExtraArgs = false
	return m
}

// AllowExtraArgs configures the matcher to ignore any
// extra arguments in a given shell.Command that don't match the given matchers
func (m *CmdMatcher) AllowExtraArgs() *CmdMatcher {
	m.allowExtraArgs = true
	return m
}

// AsCmd returns a representation of this matcher as a shell.Command object,
// useful for displaying in console output
func (m *CmdMatcher) AsCmd() shell.Command {
	var cmd shell.Command

	cmd.Prog = m.progConstraint.String()

	cmd.Args = make([]string, len(m.argConstraints))
	for i, a := range m.argConstraints {
		cmd.Args[i] = a.String()
	}
	if m.allowExtraArgs {
		cmd.Args = append(cmd.Args, "<...>")
	}

	cmd.Env = make([]string, len(m.envConstraints))
	for i, constraint := range m.envConstraints {
		cmd.Env[i] = fmt.Sprintf("%s=%s", constraint.name, constraint.valueMatcher.String())
	}
	if m.allowExtraEnvVars {
		cmd.Env = append(cmd.Env, "<...>")
	}

	cmd.Dir = m.dirConstraint.String()

	return cmd
}

// matchesArgs checks that the given shell.Command's arguments match this matcher's constraints
func (m *CmdMatcher) matchesArgs(cmd shell.Command) bool {
	if len(cmd.Args) < len(m.argConstraints) {
		// not enough args to satisfy our constraints
		return false
	}
	if len(cmd.Args) > len(m.argConstraints) && !m.allowExtraArgs {
		// too many args to satisfy our constraints
		return false
	}
	for i, arg := range cmd.Args {
		if i >= len(m.argConstraints) {
			break
		}
		constraint := m.argConstraints[i]
		if !constraint.Matches(arg) {
			return false
		}
	}

	return true
}

// matchesEnvVars checks that the given shell.Command's arguments match this matcher's constraints
func (m *CmdMatcher) matchesEnvVars(cmd shell.Command) bool {
	// convert list of "NAME=VALUE" pairs to map of "NAME" => "VALUE" associations
	// as a side-effect this will remove duplicate env vars in the slice (last wins)
	cmdVars := make(map[string]string, len(cmd.Env))
	for _, pair := range cmd.Env {
		name, value := splitEnvPair(pair)
		cmdVars[name] = value
	}

	if len(cmdVars) < len(m.envConstraints) {
		// not enough env vars to satisfy our constraints
		return false
	}

	if len(cmdVars) > len(m.envConstraints) {
		// more variables than constraints...
		if !m.allowExtraEnvVars {
			// and we don't allow extras, so fail the match
			return false
		}
	}

	// iterate through our constraints and check that each one is matched
	// by the corresponding env var
	for _, constraint := range m.envConstraints {
		value, existsInMap := cmdVars[constraint.name]
		if !existsInMap {
			// there's no env var that matches this constraint
			return false
		}
		if !constraint.valueMatcher.Matches(value) {
			// we have an env var but the value doesn't match the constraint
			return false
		}
	}

	return true
}

// splitEnvPair splits an environment key value pair "NAME=VALUE" into
// two strings, "NAME" and "VALUE".
// Eg.
// splitEnvPair("FOO=BAR") -> "FOO", "BAR"
// splitEnvPair("FOO") -> "FOO", ""
// splitEnvPair("A=B=C") -> "A", "B=C"
// splitEnvPair("=") -> "", ""
func splitEnvPair(pair string) (name string, value string) {
	tokens := strings.SplitN(pair, "=", 2)
	name = tokens[0]
	value = ""
	if len(tokens) > 1 {
		value = tokens[1]
	}
	return
}

// toGeneric converts a slice of strings to a slice of interface{}
func toGeneric(a []string) []interface{} {
	generic := make([]interface{}, len(a))
	for i := range a {
		generic[i] = a[i]
	}
	return generic
}
