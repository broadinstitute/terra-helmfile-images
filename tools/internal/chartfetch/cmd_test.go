package chartfetch

import (
	"errors"
	"fmt"
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/shellmock"
	"github.com/stretchr/testify/assert"
	"regexp"
	"strings"
	"testing"
)

// Integration test for `chartfetch`
// Executes the Cobra command with the given arguments and verifies that the
// correct "helm fetch" command is executed under the hood.
func TestFetch(t *testing.T) {
	tmpDir := t.TempDir()

	testCases := []struct {
		description   string
		args          []string
		setupMocks    func(t *testing.T, m *shellmock.MockRunner)
		expectedError *regexp.Regexp
	}{
		{
			description:   "no arguments",
			expectedError: regexp.MustCompile(`accepts 1 arg\(s\), received 0`),
		},
		{
			description:   "missing -d flag",
			args:          args("terra-helm/leonardo -v 1.2.3"),
			expectedError: regexp.MustCompile(`required flag\(s\) "download-dir" not set`),
		},
		{
			description:   "missing -v flag",
			args:          args("terra-helm/leonardo -d /does/not/exist"),
			expectedError: regexp.MustCompile(`required flag\(s\) "version" not set`),
		},
		{
			description: "should exit without downloading if directory exists",
			args:        args("terra-helm/leonardo -v 1.2.3 -d %s", tmpDir),
		},
		{
			description: "should download if directory does not exist",
			args:        args("terra-helm/leonardo -v 1.2.3 -d %s/download-dir", tmpDir),
			setupMocks: func(t *testing.T, m *shellmock.MockRunner) {
				m.ExpectCmdFmt(t, "helm fetch terra-helm/leonardo --untar -d %s/download-dir", tmpDir)
			},
		},
		{
			description: "should return an error if the helm fetch command fails",
			args:        args("terra-helm/leonardo -v 1.2.3 -d %s/download-dir", tmpDir),
			setupMocks: func(t *testing.T, m *shellmock.MockRunner) {
				m.ExpectCmdFmt(t, "helm fetch terra-helm/leonardo --untar -d %s/download-dir", tmpDir).
					Return(errors.New("command failed because reasons"))
			},
			expectedError: regexp.MustCompile("command failed because reasons"),
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.description, func(t *testing.T) {
			// Create a new mock runner and inject it by setting shellRunner package-level variable to mock
			var mockRunner = shellmock.DefaultMockRunner()
			var originalRunner = shellRunner
			shellRunner = mockRunner
			defer func() { shellRunner = originalRunner }()

			// Setup
			if testCase.setupMocks != nil {
				testCase.setupMocks(t, mockRunner)
			}

			// Create cobra command, set args, & execute it
			cobraCmd, err := newCobraCommand()
			if err != nil {
				t.Errorf("Unexpected error constructing Cobra command: %v", err)
			}
			cobraCmd.SetArgs(testCase.args)
			err = cobraCmd.Execute()

			// Verify error was correctly returned if expected
			if testCase.expectedError == nil {
				assert.Nil(t, err, "Expected command to execute successfully, but it returned an error: %v", err)
			} else {
				assert.NotNil(t, err, "Expected command to return error, but it did not")
				assert.Regexp(t, testCase.expectedError, err, "Error mismatch: %v", err)
			}

			// Verify all expected shell commands were called
			mockRunner.AssertExpectations(t)
		})
	}
}

// Silly convenience function for building argument list in test cases
func args(format string, a ...interface{}) []string {
	formatted := fmt.Sprintf(format, a...)
	return strings.Fields(formatted)
}
