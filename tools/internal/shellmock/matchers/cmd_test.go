package matchers

import (
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/shell"
	"github.com/stretchr/testify/assert"
	"regexp"
	"strings"
	"testing"
)

func TestCmdMatches(t *testing.T) {
	type testCase struct {
		matcherName     string
		matcher         *CmdMatcher
		expectedResults map[string]bool
	}

	testCmds := map[string]shell.Command{
		"empty": {},
		"ls": {
			Prog: "ls",
		},
		"lsWithArg": {
			Prog: "ls",
			Args: []string{"/tmp"},
		},
		"lsWithTwoArgs": {
			Prog: "ls",
			Args: []string{"-a", "/tmp"},
		},
		"lsWithFourArgs": {
			Prog: "ls",
			Args: []string{"-l", "-a", "-t", "/tmp"},
		},
		"lsWithEnvVar": {
			Prog: "ls",
			Env:  []string{"HOME=/root"},
		},
		"lsWithEnvVars": {
			Prog: "ls",
			Env:  []string{"HOME=/root", "FOO=BAR", "BAZ=QUUX"},
		},
		"lsWithTmpDir": {
			Prog: "ls",
			Dir:  "/tmp",
		},
		"lsWithAll": {
			Prog: "ls",
			Args: []string{"-l", "-a", "-t", "."},
			Env:  []string{"HOME=/root", "FOO=BAZ", "A=B", "X=Y"},
			Dir:  "/tmp",
		},
		"echo": {
			Prog: "echo",
		},
		"echoWithArgs": {
			Prog: "echo",
			Args: []string{"-n", "hello", "world"},
		},
		"echoWithArgsAndHomeEnvVar": {
			Prog: "echo",
			Args: []string{"-n", "hello", "world"},
			Env:  []string{"HOME=/root"},
		},
		"echoWithArgsAndTwoFooEnvVarsBazLast": {
			Prog: "echo",
			Args: []string{"-n", "hello", "world"},
			Env:  []string{"HOME=/root", "FOO=BAR", "BEEP=BOOP", "FOO=BAZ"},
		},
		"echoWithArgsAndTwoFooEnvVarsBarLast": {
			Prog: "echo",
			Args: []string{"-n", "hello", "world"},
			Env:  []string{"HOME=/root", "FOO=BAZ", "BEEP=BOOP", "FOO=BAR"},
		},
		"echoWithSlashDir": {
			Prog: "echo",
			Dir:  "/",
		},
		"echoWithTmpDir": {
			Prog: "echo",
			Dir:  "/tmp",
		},
		"echoWithAll": {
			Prog: "echo",
			Args: []string{"-n", "hello", "world"},
			Env:  []string{"HOME=/root", "FOO=BAZ", "BEEP=BOOP", "FOO=BAR"},
			Dir:  "/tmp",
		},
	}

	testCases := []testCase{
		{
			matcherName: "any",
			matcher:     AnyCmd(),
			expectedResults: map[string]bool{
				"empty":                               true,
				"ls":                                  true,
				"lsWithArg":                           true,
				"lsWithTwoArgs":                       true,
				"lsWithFourArgs":                      true,
				"lsWithEnvVar":                        true,
				"lsWithEnvVars":                       true,
				"lsWithTmpDir":                        true,
				"lsWithAll":                           true,
				"echo":                                true,
				"echoWithArgs":                        true,
				"echoWithTmpDir":                      true,
				"echoWithSlashDir":                    true,
				"echoWithArgsAndHomeEnvVar":           true,
				"echoWithArgsAndTwoFooEnvVarsBarLast": true,
				"echoWithArgsAndTwoFooEnvVarsBazLast": true,
				"echoWithAll":                         true,
			},
		},
		{
			matcherName: "progIsLsLiteral",
			matcher:     AnyCmd().WithProg("ls"),
			expectedResults: map[string]bool{
				"ls":             true,
				"lsWithArg":      true,
				"lsWithTwoArgs":  true,
				"lsWithFourArgs": true,
				"lsWithEnvVar":   true,
				"lsWithEnvVars":  true,
				"lsWithTmpDir":   true,
				"lsWithAll":      true,
			},
		},
		{
			matcherName: "progIsLsEqualsMatcher",
			matcher:     AnyCmd().WithProg(Equals("ls")),
			expectedResults: map[string]bool{
				"ls":             true,
				"lsWithArg":      true,
				"lsWithTwoArgs":  true,
				"lsWithFourArgs": true,
				"lsWithEnvVar":   true,
				"lsWithEnvVars":  true,
				"lsWithTmpDir":   true,
				"lsWithAll":      true,
			},
		},
		{
			matcherName: "progStartsWithL",
			matcher:     AnyCmd().WithProg(MatchesRegexp(regexp.MustCompile("^l"))),
			expectedResults: map[string]bool{
				"ls":             true,
				"lsWithArg":      true,
				"lsWithTwoArgs":  true,
				"lsWithFourArgs": true,
				"lsWithEnvVar":   true,
				"lsWithEnvVars":  true,
				"lsWithTmpDir":   true,
				"lsWithAll":      true,
			},
		},
		{
			matcherName: "progIsNotEchoLiteral",
			matcher:     AnyCmd().WithProg(Not(Equals("echo"))),
			expectedResults: map[string]bool{
				"empty":          true,
				"ls":             true,
				"lsWithArg":      true,
				"lsWithTwoArgs":  true,
				"lsWithFourArgs": true,
				"lsWithEnvVar":   true,
				"lsWithEnvVars":  true,
				"lsWithTmpDir":   true,
				"lsWithAll":      true,
			},
		},
		{
			matcherName: "progIsLsStrictArgs",
			matcher:     AnyCmd().WithProg("ls").FailIfExtraArgs(),
			expectedResults: map[string]bool{
				"ls":            true,
				"lsWithEnvVar":  true,
				"lsWithEnvVars": true,
				"lsWithTmpDir":  true,
			},
		},
		{
			matcherName: "progIsLsStrictEnvVars",
			matcher:     AnyCmd().WithProg("ls").FailIfExtraEnvVars(),
			expectedResults: map[string]bool{
				"ls":             true,
				"lsWithArg":      true,
				"lsWithTwoArgs":  true,
				"lsWithFourArgs": true,
				"lsWithTmpDir":   true,
			},
		},
		{
			matcherName: "hasFooEnvVarWithAnyValue",
			matcher:     AnyCmd().WithEnvVar("FOO", AnyString()),
			expectedResults: map[string]bool{
				"lsWithEnvVars":                       true,
				"lsWithAll":                           true,
				"echoWithArgsAndTwoFooEnvVarsBarLast": true,
				"echoWithArgsAndTwoFooEnvVarsBazLast": true,
				"echoWithAll":                         true,
			},
		},
		{
			matcherName: "hasFoo=BarEnvVar",
			matcher:     AnyCmd().WithEnvVar("FOO", "BAR"),
			expectedResults: map[string]bool{
				"lsWithEnvVars":                       true,
				"echoWithArgsAndTwoFooEnvVarsBarLast": true,
				"echoWithAll":                         true,
			},
		},
		{
			matcherName: "hasFoo=BazEnvVar",
			matcher:     AnyCmd().WithEnvVar("FOO", "BAZ"),
			expectedResults: map[string]bool{
				"lsWithAll":                           true,
				"echoWithArgsAndTwoFooEnvVarsBazLast": true,
			},
		},
		{
			matcherName: "hasFooStartsWithBAEnvVar",
			matcher:     AnyCmd().WithEnvVar("FOO", regexp.MustCompile("^BA")),
			expectedResults: map[string]bool{
				"lsWithEnvVars":                       true,
				"lsWithAll":                           true,
				"echoWithArgsAndTwoFooEnvVarsBarLast": true,
				"echoWithArgsAndTwoFooEnvVarsBazLast": true,
				"echoWithAll":                         true,
			},
		},
		{
			matcherName: "hasHomeIsRootEnvVarOnly",
			matcher:     AnyCmd().WithEnvVar("HOME", "/root").FailIfExtraEnvVars(),
			expectedResults: map[string]bool{
				"lsWithEnvVar":              true,
				"echoWithArgsAndHomeEnvVar": true,
			},
		},
		{
			matcherName: "hasHomeIsRootEnvVar",
			matcher:     AnyCmd().WithEnvVar("HOME", "/root"),
			expectedResults: map[string]bool{
				"lsWithEnvVar":                        true,
				"lsWithEnvVars":                       true,
				"lsWithAll":                           true,
				"echoWithArgsAndHomeEnvVar":           true,
				"echoWithArgsAndTwoFooEnvVarsBarLast": true,
				"echoWithArgsAndTwoFooEnvVarsBazLast": true,
				"echoWithAll":                         true,
			},
		},
		{
			matcherName: "hasHomeIsRootAndFooIsBarEnvVars",
			matcher:     AnyCmd().WithEnvVar("HOME", "/root").WithEnvVar("FOO", "BAR"),
			expectedResults: map[string]bool{
				"lsWithEnvVars":                       true,
				"echoWithArgsAndTwoFooEnvVarsBarLast": true,
				"echoWithAll":                         true,
			},
		},
		{
			matcherName: "hasHomeIsRootAndFooStartsWithBAEnvVars",
			matcher:     AnyCmd().WithEnvVar("HOME", "/root").WithEnvVar("FOO", regexp.MustCompile("BA")),
			expectedResults: map[string]bool{
				"lsWithEnvVars":                       true,
				"lsWithAll":                           true,
				"echoWithArgsAndTwoFooEnvVarsBarLast": true,
				"echoWithArgsAndTwoFooEnvVarsBazLast": true,
				"echoWithAll":                         true,
			},
		},
		{
			matcherName: "firstArgContainsADash",
			matcher:     AnyCmd().WithArgAt(0, Contains("-")),
			expectedResults: map[string]bool{
				"lsWithTwoArgs":                       true,
				"lsWithFourArgs":                      true,
				"lsWithAll":                           true,
				"echoWithArgs":                        true,
				"echoWithArgsAndHomeEnvVar":           true,
				"echoWithArgsAndTwoFooEnvVarsBazLast": true,
				"echoWithArgsAndTwoFooEnvVarsBarLast": true,
				"echoWithAll":                         true,
			},
		},
		{
			matcherName: "firstArgContainsADashAndSecondIsTmp",
			matcher:     AnyCmd().WithArgAt(0, Contains("-")).WithArgAt(1, "/tmp"),
			expectedResults: map[string]bool{
				"lsWithTwoArgs": true,
			},
		},
		{
			matcherName: "withSlashDir",
			matcher:     AnyCmd().WithDir("/"),
			expectedResults: map[string]bool{
				"echoWithSlashDir": true,
			},
		},
		{
			matcherName: "withTmpDir",
			matcher:     AnyCmd().WithDir("/tmp"),
			expectedResults: map[string]bool{
				"lsWithTmpDir":   true,
				"lsWithAll":      true,
				"echoWithTmpDir": true,
				"echoWithAll":    true,
			},
		},
		{
			matcherName: "withEitherTmpOrSlashDir",
			matcher: AnyCmd().
				WithDir(MatchesPredicate("either /tmp or /", func(dir string) bool {
					return dir == "/tmp" || dir == "/"
				})),
			expectedResults: map[string]bool{
				"lsWithTmpDir":     true,
				"lsWithAll":        true,
				"echoWithTmpDir":   true,
				"echoWithSlashDir": true,
				"echoWithAll":      true,
			},
		},
		{
			matcherName: "withTmpDirAndFooEnvVar",
			matcher:     AnyCmd().WithDir("/tmp").WithEnvVar("FOO", AnyString()),
			expectedResults: map[string]bool{
				"lsWithAll":   true,
				"echoWithAll": true,
			},
		},
		{
			matcherName: "withExactArgsTmp",
			matcher:     AnyCmd().WithExactArgs("/tmp"),
			expectedResults: map[string]bool{
				"lsWithArg": true,
			},
		},
		{
			matcherName: "cmdWithArgsLsNoArgs",
			matcher:     CmdWithArgs("ls"),
			expectedResults: map[string]bool{
				"ls":           true,
				"lsWithTmpDir": true,
			},
		},
		{
			matcherName: "cmdWithArgsLsATmp",
			matcher:     CmdWithArgs("ls", "-a", "/tmp"),
			expectedResults: map[string]bool{
				"lsWithTwoArgs": true,
			},
		},
		{
			matcherName: "cmdWithEnvLs",
			matcher:     CmdWithEnv("ls"),
			expectedResults: map[string]bool{
				"ls":           true,
				"lsWithTmpDir": true,
			},
		},
		{
			matcherName: "cmdWithEnvLsOneEnvVar",
			matcher:     CmdWithEnv("HOME=/root", "ls"),
			expectedResults: map[string]bool{
				"lsWithEnvVar": true,
			},
		},
		{
			matcherName: "cmdWithEnvLsOneEnvVar",
			matcher:     CmdWithEnv("FOO=BAR", "BAZ=QUUX", "HOME=/root", "ls"),
			expectedResults: map[string]bool{
				"lsWithEnvVars": true,
			},
		},
		{
			matcherName: "cmdWithEnvEmpty",
			matcher:     CmdWithEnv(),
			expectedResults: map[string]bool{
				"empty": true,
			},
		},
		{
			matcherName:     "cmdWithEnvNoProg",
			matcher:         CmdWithEnv("FOO=BAR"),
			expectedResults: map[string]bool{
				// should match nothing
			},
		},
		{
			matcherName: "cmdWithProgAndArg",
			matcher:     CmdWithEnv("ls", "/tmp"),
			expectedResults: map[string]bool{
				"lsWithArg": true,
			},
		},
		{
			matcherName: "cmdWithEnvProgAndArgs",
			matcher:     CmdWithEnv("ls", "-l", "-a", "-t", "/tmp"),
			expectedResults: map[string]bool{
				"lsWithFourArgs": true,
			},
		},
		{
			matcherName: "cmdWithOutOfOrderArgMatchers",
			matcher:     AnyCmd().WithArgAt(3, Contains("tmp")).WithArgAt(0, "-l"),
			expectedResults: map[string]bool{
				"lsWithFourArgs": true,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.matcherName, func(t *testing.T) {
			// Make sure there are no typos in the expectedResults map
			for cmdName := range tc.expectedResults {
				if _, exists := testCmds[cmdName]; !exists {
					t.Errorf("Matcher %s is configured to use an unknown test command: %s", tc.matcherName, cmdName)
				}
			}

			// Iterate through defined commands and test them against the matcher
			for cmdName, cmd := range testCmds {
				t.Run(cmdName, func(t *testing.T) {
					expected := tc.expectedResults[cmdName]
					assert.Equal(t, expected, tc.matcher.Matches(cmd), "%s.Matches(%s) should return %v (command: %#v)", tc.matcherName, cmdName, expected, cmd)
				})
			}
		})
	}
}

func TestAsCmd(t *testing.T) {
	type testCase struct {
		name     string
		expected shell.Command
		actual   shell.Command
	}

	testCases := []testCase{{
		name:   "any",
		actual: AnyCmd().AsCmd(),
		expected: shell.Command{
			Prog: "<any>",
			Args: []string{"<...>"},
			Env:  []string{"<...>"},
			Dir:  "<any>",
		},
	}, {
		name:   "prog only",
		actual: CmdWithArgs("ls").AsCmd(),
		expected: shell.Command{
			Prog: "ls",
			Args: []string{},
			Env:  []string{},
			Dir:  "<any>",
		},
	}, {
		name:   "prog env and arg",
		actual: CmdWithEnv("FOO=BAR", "ls", "/home").AsCmd(),
		expected: shell.Command{
			Prog: "ls",
			Args: []string{"/home"},
			Env:  []string{"FOO=BAR"},
			Dir:  "<any>",
		},
	}, {
		name: "prog flexible env and args",
		actual: AnyCmd().
			WithProg("echo").
			WithArgAt(0, Contains("ello")).
			WithArgAt(1, regexp.MustCompile("^wo")).
			WithEnvVar("TMP", AnyString()).
			WithEnvVar("HOME",
				MatchesPredicate("ends with slash",
					func(dir string) bool {
						return strings.HasSuffix(dir, "/")
					},
				)).
			AsCmd(),
		expected: shell.Command{
			Prog: "echo",
			Args: []string{"contains(ello)", "match(/^wo/)", "<...>"},
			Env:  []string{"TMP=<any>", "HOME=<ends with slash>", "<...>"},
			Dir:  "<any>",
		},
	}}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			assert.Equal(t, testCase.expected, testCase.actual)
		})
	}
}

func TestCmdFromFmt(t *testing.T) {
	matcher := CmdFromFmt("HOME=%s FOO=BAR ls -al %s", "/root", "/tmp")
	cmd := shell.Command{
		Prog: "ls",
		Args: []string{"-al", "/tmp"},
		Env: []string{"HOME=/root", "FOO=BAR"},
	}
	assert.True(t, matcher.Matches(cmd))

	expected := shell.Command{
		Prog: "ls",
		Args: []string{"-al", "/tmp"},
		Env: []string{"HOME=/root", "FOO=BAR"},
		Dir: "<any>",
	}
	assert.Equal(t, expected, matcher.AsCmd())
}