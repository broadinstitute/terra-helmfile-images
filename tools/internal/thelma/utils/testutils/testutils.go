package testutils

import (
	"fmt"
	"os"
	"strings"
)

// Args convenience function to generate tokenized argument list from format string w/ args
//
// Eg. args("-e   %s", "dev") -> []string{"-e", "dev"}
func Args(format string, a ...interface{}) []string {
	formatted := fmt.Sprintf(format, a...)
	return strings.Fields(formatted)
}

// Cwd convenience function to return current working directory
func Cwd() string {
	dir, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	return dir
}
