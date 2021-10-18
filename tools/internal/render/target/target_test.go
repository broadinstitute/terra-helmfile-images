package target

import (
	"github.com/stretchr/testify/assert"
	"math/rand"
	"os"
	"path"
	"regexp"
	"strings"
	"testing"
)

func TestLoadReleaseTargets(t *testing.T) {
	testCases := []struct {
		description string
		directoryStructure []string
		expectedError string
		expectedTargets map[string]ReleaseTarget
	}{
		{
			description: "missing root directory",
			expectedError: "config directory does not exist",
		},
		{
			description: "missing clusters/ directory",
			directoryStructure: []string{
				"environments/live/dev.yaml",
			},
			expectedError: "cluster config directory does not exist",
		},
		{
			description: "missing environments/ directory",
			directoryStructure: []string{
				"clusters/terra/terra-dev.yaml",
			},
			expectedError: "environment config directory does not exist",
		},
		{
			description: "no clusters in directory",
			directoryStructure: []string{
				"clusters/live/",
				"environments/live/dev.yaml",
			},
			expectedError: "no cluster configs found",
		},
		{
			description: "no environments in directory",
			directoryStructure: []string{
				"clusters/terra/terra-dev.yaml",
				"environments/",
			},
			expectedError: "no environment configs found",
		},
		{
			description: "two environments same name",
			directoryStructure: []string{
				"clusters/terra/terra-dev.yaml",
				"environments/live/dev.yaml",
				"environments/personal/dev.yaml",
			},
			expectedError: `environment name conflict dev \(personal\) and dev \(live\)`,
		},
		{
			description: "two clusters same name",
			directoryStructure: []string{
				"clusters/terra/dev.yaml",
				"clusters/tdr/dev.yaml",
				"environments/live/dev.yaml",
			},
			expectedError: `cluster name conflict dev \(terra\) and dev \(tdr\)`,
		},
		{
			description: "env and cluster same name",
			directoryStructure: []string{
				"clusters/terra/dev.yaml",
				"environments/live/dev.yaml",
			},
			expectedError: "cluster name dev conflicts with environment name dev",
		},
		{
			description: "3 envs, 2 clusters",
			directoryStructure: []string{
				"clusters/terra/terra-dev.yaml",
				"clusters/terra/terra-alpha.yaml",
				"environments/live/dev.yaml",
				"environments/live/perf.yaml",
				"environments/live/alpha.yaml",
			},
			expectedTargets: map[string]ReleaseTarget{
				"terra-dev": NewCluster("terra-dev", "terra"),
				"terra-alpha": NewCluster("terra-alpha", "terra"),
				"dev": NewEnvironment("dev", "live"),
				"perf": NewEnvironment("perf", "live"),
				"alpha": NewEnvironment("alpha", "live"),
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.description, func(t *testing.T) {
			testDir := path.Join(t.TempDir(), "configs")
			if err := populateTestFiles(testDir, testCase.directoryStructure); err != nil {
				t.Fatal(err)
			}
			releaseTargets, err := LoadReleaseTargets(testDir)

			if testCase.expectedError != "" {
				if !assert.Error(t, err, "Expected error matching: %v", testCase.expectedError) {
					return
				}
				assert.Regexp(t, regexp.MustCompile(testCase.expectedError), err.Error())
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			assert.Equal(t, testCase.expectedTargets, releaseTargets)
		})
	}
}

// populateTestFiles given a root directory and list of files/directories, populate the
// directory with empty test files
// eg. populateTestFiles("/tmp/my-test-dir", []string{"foo/bar.yaml", "foo/bar/baz.yaml", "quux/"})
// will create:
//
// /tmp/
//   my-test-dir/
//     foo/
//       bar.yaml
//       bar/
//         baz.yaml
//     quux/
func populateTestFiles(rootDir string, files []string) error {
	for _, entry := range files {
		file := path.Join(rootDir, entry)
		if strings.HasSuffix(file, "/") { // directory
			if err := os.MkdirAll(file, 0755); err != nil {
				return err
			}
		} else { // file
			if err := os.MkdirAll(path.Dir(file), 0755); err != nil {
				return err
			}
			if err := os.WriteFile(file, []byte("# test file"), 0644); err != nil {
				return err
			}
		}
	}

	return nil
}

func TestSortReleaseTargets(t *testing.T) {
	testCases := []struct{
		description string
		input []ReleaseTarget
		expected []ReleaseTarget
	}{
		{
			description: "empty",
			input: make([]ReleaseTarget, 0),
			expected: make([]ReleaseTarget, 0),
		},
		{
			description: "single",
			input: []ReleaseTarget{NewEnvironment("dev", "live")},
			expected: []ReleaseTarget{NewEnvironment("dev", "live")},
		},
		{
			description: "two",
			input: []ReleaseTarget{
				NewEnvironment("dev", "live"),
				NewEnvironment("alpha", "live"),
			},
			expected: []ReleaseTarget{
				NewEnvironment("alpha", "live"),
				NewEnvironment("dev", "live"),
			},
		},
		{
			description: "many",
			input: []ReleaseTarget{
				NewEnvironment("staging", "live"),
				NewEnvironment("dev", "live"),
				NewCluster("tdr-staging", "tdr"),
				NewCluster("terra-perf", "terra"),
				NewEnvironment("prod", "live"),
				NewCluster("terra-alpha", "terra"),
				NewEnvironment("perf", "live"),
				NewEnvironment("alpha", "live"),
				NewEnvironment("jdoe", "personal"),
				NewCluster("tdr-alpha", "tdr"),
			},
			expected: []ReleaseTarget{
				NewCluster("tdr-alpha", "tdr"),
				NewCluster("tdr-staging", "tdr"),
				NewCluster("terra-alpha", "terra"),
				NewCluster("terra-perf", "terra"),
				NewEnvironment("alpha", "live"),
				NewEnvironment("dev", "live"),
				NewEnvironment("perf", "live"),
				NewEnvironment("prod", "live"),
				NewEnvironment("staging", "live"),
				NewEnvironment("jdoe", "personal"),
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.description, func(t *testing.T) {
			input := testCase.input
			expected := testCase.expected

			rand.Shuffle(len(input), func(i int, j int){
				input[i], input[j] = input[j], input[i]
			})

			SortReleaseTargets(input)
			assert.Equal(t, expected, input)
		})
	}
}