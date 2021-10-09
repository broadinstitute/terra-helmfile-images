package matchers

import (
	"fmt"
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/shell"
	"strings"
)

// CmdMatcher is for evaluating whether a shell.Command matches configured
// constraints.
type CmdMatcher struct {
	progConstraint StringMatcher
	argConstraints []StringMatcher
	envConstraints map[string]StringMatcher
	dirConstraint  StringMatcher
	// TODO check pristineEnv? (haven't encountered a use case where it was necessary to test)
	allowExtraEnvVars bool
	allowExtraArgs    bool
}

// newCmdMatcher initializes a new command matchers with sensible defaults
func newCmdMatcher() *CmdMatcher {
	matcher := new(CmdMatcher)
	matcher.progConstraint = None()
	matcher.envConstraints = make(map[string]StringMatcher)
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
func CmdWithArgs(prog string, args... string) *CmdMatcher {
	matcher := newCmdMatcher()
	matcher.WithProg(prog)
	matcher.WithExactArgs(toGeneric(args)...)
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
func (m *CmdMatcher) WithExactArgs(matchers... interface{}) *CmdMatcher {
	args := make([]StringMatcher, len(matchers))
	for i, matcher := range matchers {
		args[i] = toStringMatcher(matcher)
	}
	m.argConstraints = args
	m.FailIfExtraArgs()
	return m
}

// WithArg configures the matcher to expect an argument matching the given matcher.
// May be called multiple times to match arguments at sequential indexes.
//
// eg. matcher.WithArg(AnyString()).
//       WithArg("exact string").
//       WithArg(MatchesRegexp(Regexp.MustCompile("foo.*")))
func (m *CmdMatcher) WithArg(matcher interface{}) *CmdMatcher {
	m.argConstraints = append(m.argConstraints, toStringMatcher(matcher))
	return m
}

// WithExactEnvVars configures the matcher to expect the exact given
// set of env vars, supplied as NAME=VALUE pairs.
//
// eg. matcher.WithExactEnvVars("FOO=BAR", "BAZ=QUUX")
func (m *CmdMatcher) WithExactEnvVars(pairs... string) *CmdMatcher {
	envVars := make(map[string]StringMatcher, len(pairs))
	for _, pair := range pairs {
		name, value := splitEnvPair(pair)
		envVars[name] = Equals(value)
	}
	m.envConstraints = envVars
	m.FailIfExtraEnvVars()
	return m
}

// WithEnvVar configures the matcher to expect an env var matching the given matcher.
// May be called multiple times to match different env variable names.
//
// eg. matcher.WithEnvVar("FOO", "BAR").WithEnvVar("BAZ", HasPrefix("FOO_"))
func (m *CmdMatcher) WithEnvVar(name string, matcher interface{}) *CmdMatcher {
	m.envConstraints[name] = toStringMatcher(matcher)
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

	// check Args
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

	// check Env
	if len(cmd.Env) < len(m.envConstraints) {
		// not enough env vars to satisfy our constraints
		return false
	}

	for _, pair := range cmd.Env {
		tokens := strings.SplitN(pair, "=", 2)
		name := tokens[0]
		value := ""
		if len(tokens) > 1 {
			value = tokens[1]
		}

		constraint, existsInMap := m.envConstraints[name]
		if !existsInMap && !m.allowExtraEnvVars {
			// extraneous env var
			return false
		}
		if !constraint.Matches(value) {
			return false
		}
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
	i := 0
	for name, val := range m.envConstraints {
		cmd.Env[i] = fmt.Sprintf("%s=%s", name, val.String())
		i++
	}
	if m.allowExtraEnvVars {
		cmd.Env = append(cmd.Env, "<...>")
	}

	cmd.Dir = m.dirConstraint.String()

	return cmd
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