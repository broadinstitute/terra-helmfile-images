package config

import (
	"github.com/stretchr/testify/assert"
	"reflect"
	"testing"
)

func TestConfigKeyNamesMatchYamlTags(t *testing.T) {
	dataType := reflect.TypeOf(Data{})
	keysType := reflect.TypeOf(Keys)

	assert.Positive(t, dataType.NumField(), "config.Data type should have at least one field")
	assert.Equal(t, dataType.NumField(), keysType.NumField(), "config.Data and config.Keys should have same number of fields")

	// Iterate through the fields of the config data type and make sure there is a corresponding Key field
	for i := 0; i < dataType.NumField(); i++ {
		fieldName := dataType.Field(i).Name
		yamlTag := dataType.Field(i).Tag.Get("yaml")

		if yamlTag == "" {
			t.Fatalf("config.Data field %s is missing a yaml tag", fieldName)
		}

		keyField := reflect.ValueOf(Keys).FieldByName(fieldName)
		if keyField.IsZero() {
			t.Fatalf("config.Keys should have field %s corresponding to config.Data field %s, but it does not", fieldName, fieldName)
		}

		actual := keyField.String()
		assert.Equal(t, yamlTag, actual, "Expected Keys.%s to equal config.Data.%s yaml tag %s, got %s", fieldName, fieldName, yamlTag, actual)
	}
}
