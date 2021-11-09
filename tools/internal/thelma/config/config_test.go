package config

import (
	"github.com/stretchr/testify/assert"
	"reflect"
	"testing"
)

func TestConfigKeyNamesMatchYamlTags(t *testing.T) {
	dataType := reflect.TypeOf(ConfigData{})
	keysType := reflect.TypeOf(ConfigKeys)

	assert.Positive(t, dataType.NumField(), "config.ConfigData type should have at least one field")
	assert.Equal(t, dataType.NumField(), keysType.NumField(), "config.ConfigData and config.ConfigKeys should have same number of fields")

	// Iterate through the fields of the config data type and make sure there is a corresponding Key field
	for i := 0; i < dataType.NumField(); i++ {
		fieldName := dataType.Field(i).Name
		yamlTag := dataType.Field(i).Tag.Get("yaml")

		if yamlTag == "" {
			t.Fatalf("config.ConfigData field %s is missing a yaml tag", fieldName)
		}

		keyField := reflect.ValueOf(ConfigKeys).FieldByName(fieldName)
		if keyField.IsZero() {
			t.Fatalf("config.ConfigKeys should have field %s corresponding to config.ConfigData field %s, but it does not", fieldName, fieldName)
		}

		actual := keyField.String()
		assert.Equal(t, yamlTag, actual, "Expected ConfigKeys.%s to equal config.ConfigData.%s yaml tag %s, got %s", fieldName, fieldName, yamlTag, actual)
	}
}
