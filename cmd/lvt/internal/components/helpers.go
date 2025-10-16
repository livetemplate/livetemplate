package components

import (
	"gopkg.in/yaml.v3"
)

// unmarshalYAML is a helper function to unmarshal YAML data
func unmarshalYAML(data []byte, v interface{}) error {
	return yaml.Unmarshal(data, v)
}
