package matchers

import (
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/shell"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestMatches(t *testing.T) {
	type testCmd struct {
		cmd shell.Command
		name string
	}

	type testCase struct {
		matcherName     string
		matcher         *CmdMatcher
		expectedResults map[string]bool
	}

	testCmds := []testCmd{
		{
			name: "empty",
			cmd: shell.Command{},
		},{
			name: "ls",
			cmd: shell.Command{Prog:"ls"},
		},{
			name: "lsWithArg",
			cmd: shell.Command{Prog:"ls", Args: []string{"/tmp"}},
		},{
			name: "lsWithArgs",
			cmd: shell.Command{Prog:"ls", Args: []string{"-l", "-a", "-t", "/tmp"}},
		},{
			name: "lsWithEnvVar",
			cmd: shell.Command{Prog:"ls", Env: []string{"HOME=/root"}},
		},{
			name: "lsWithEnvVars",
			cmd: shell.Command{Prog:"ls", Env: []string{"HOME=/root", "FOO=BAR", "BAZ=QUUX"}},
		},{
			name: "lsWithDir",
			cmd: shell.Command{Prog:"ls", Dir: "/tmp"},
		},{
			name: "lsWithAll",
			cmd: shell.Command{
				Prog:"ls",
				Args: []string{"-l", "-a", "-t", "."},
				Env: []string{"HOME=/root", "FOO=BAR", "BAZ=QUUX"},
				Dir: "/tmp",
			},
		},{
			name: "echo",
			cmd: shell.Command{Prog:"echo"},
		},{
			name: "echoWithArgs",
			cmd: shell.Command{Prog:"echo", Args:[]string{"foo","bar"}},
		},
	}

	testCases := []testCase{
		{
			matcherName: "any",
			matcher:     AnyCmd(),
			expectedResults: map[string]bool{
				"empty": true,
				"ls": true,
				"lsWithArg": true,
				"lsWithArgs": true,
				"lsWithEnvVar": true,
				"lsWithEnvVars": true,
				"lsWithDir": true,
				"lsWithAll": true,
				"echo": true,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.matcherName, func(t *testing.T) {
			for _, input := range testCmds {
				cmdName, cmd := input.name, input.cmd
				expected, ok := tc.expectedResults[cmdName]
				assert.True(t, ok, "test case %s should have an expected result for input command %s, but it does not", tc.matcherName, cmdName)

				t.Logf("%s.Matches(%s)", tc.matcherName, cmdName)
				assert.Equal(t, expected, tc.matcher.Matches(cmd), "%s.Matches(%s) should return %v (%#v)", tc.matcherName, cmdName, expected, cmd)
			}
		})
	}
}