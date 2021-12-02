package deepmerge

import (
	"fmt"
	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v3"
	"os"
)

// Deep merge all given YAML files and unmarshal the result into the given struct
func Unmarshal(result interface{}, filenames ...string) error {
	merged, err := Merge(filenames...)

	if err != nil {
		return err
	}

	err = yaml.Unmarshal(merged, result)
	if err != nil {
		return fmt.Errorf("error unmarshaling merged yaml: %v", err)
	}

	return nil
}

// Deep merge all given YAML files and return the resulting YAML as a byte array
func Merge(filenames ...string) ([]byte, error) {
	var toMerge []map[string]interface{}

	for _, filename := range filenames {
		parsed, err := unmarshalYamlFileIfExists(filename)
		if err != nil {
			return nil, err
		}
		if parsed != nil {
			toMerge = append(toMerge, parsed)
		}
	}

	merged := deepMergeAll(toMerge...)

	// Now that we've merged the content, serialize back to yaml so we can deserialize into the expected type.
	marshaled, err := yaml.Marshal(merged)
	if err != nil {
		return nil, fmt.Errorf("error marshaling yaml for deep merge: %v", err)
	}

	return marshaled, nil
}

func deepMergeAll(maps ...map[string]interface{}) map[string]interface{} {
	merged := make(map[string]interface{})
	for _, m := range maps {
		merged = deepMerge(merged, m)
	}
	return merged
}

func deepMerge(map1 map[string]interface{}, map2 map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})

	for key, v1 := range map1 {
		result[key] = v1
	}

	for key, v2 := range map2 {
		v1, exists := map1[key]
		if !exists {
			result[key] = v2
			continue
		}

		v1Map, v1IsMap := v1.(map[string]interface{})
		v2Map, v2IsMap := v2.(map[string]interface{})

		if v1IsMap && v2IsMap {
			result[key] = deepMerge(v1Map, v2Map)
		} else {
			// values in map 2 take precedence over values in map 1
			result[key] = v2
		}
	}

	return result
}

func unmarshalYamlFileIfExists(file string) (map[string]interface{}, error) {
	_, err := os.Stat(file)
	if os.IsNotExist(err) {
		log.Debug().Msgf("file %s does not exist, ignoring", file)
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("error reading file %s: %v", file, err)
	}

	content, err := os.ReadFile(file)
	if err != nil {
		return nil, fmt.Errorf("error reading file %s: %v", file, err)
	}

	var parsed map[string]interface{}
	if err := yaml.Unmarshal(content, &parsed); err != nil {
		return nil, fmt.Errorf("failed to parse yaml file %s: %v", file, err)
	}

	if parsed == nil {
		// treat empty file like missing file
		log.Debug().Msgf("file %s includes no yaml content, ignoring", file)
		return nil, nil
	}

	return parsed, nil
}
