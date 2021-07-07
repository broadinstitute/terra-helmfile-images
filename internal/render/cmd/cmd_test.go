package cmd

import (
	"testing"
)

/* Integration test verifying CLI args are parsed properly */
func TestExecute(t *testing.T) {
	var tests = []struct {
		description string
		args []string
		expectedOpts Options
		expectedErr string
	}{
		{},
	}

	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
		})
	}
}