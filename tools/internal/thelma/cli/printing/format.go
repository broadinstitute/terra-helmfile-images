package printing

import (
	"encoding/json"
	"fmt"
	"github.com/davecgh/go-spew/spew"
	"gopkg.in/yaml.v3"
	"sort"
	"strings"
)

// Type alias for functions that write data in a given format to an io.Writer
type formatter func(data interface{}) (formatted []byte, err error)

// Store output formats in a map, keyed by name
var formats = map[string]formatter{
	"yaml": formatYaml,
	"json": formatJson,
	"spew": formatSpew,
}

// List of supported formats as a string suitable for use in usage and error messages
// i.e. `"json", "text", "yaml"`
var supportedFormatsMsg = buildSupportedFormatsMsg(formats)

func formatSpew(data interface{}) ([]byte, error) {
	asString := spew.Sdump(data)
	asBytes := []byte(asString)
	return asBytes, nil
}

func formatJson(data interface{}) ([]byte, error) {
	asBytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return nil, err
	}
	return asBytes, nil
}

func formatYaml(data interface{}) ([]byte, error) {
	asBytes, err := yaml.Marshal(data)
	if err != nil {
		return nil, err
	}
	return asBytes, nil
}

func format(formatName string, data interface{}) ([]byte, error) {
	if !isSupportedFormat(formatName) {
		return nil, fmt.Errorf("unsupported format, expected one of %s: %q", supportedFormatsMsg, formatName)
	}
	fn := formats[formatName]
	return fn(data)
}

func isSupportedFormat(formatName string) bool {
	_, exists := formats[formatName]
	return exists
}

func buildSupportedFormatsMsg(_formats map[string]formatter) string {
	var quoted []string
	for name := range _formats {
		quoted = append(quoted, fmt.Sprintf("%q", name))
	}

	sort.Slice(quoted, func(i, j int) bool {
		return quoted[i] < quoted[j]
	})

	return strings.Join(quoted, ", ")
}
