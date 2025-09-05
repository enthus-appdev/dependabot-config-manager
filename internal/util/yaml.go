// Package util provides utility functions for YAML marshaling.
package util

import (
	"bytes"

	"gopkg.in/yaml.v3"
)

// MarshalYAML marshals the config with configurable indentation
func MarshalYAML(v interface{}, indent int) ([]byte, error) {
	var buf bytes.Buffer
	encoder := yaml.NewEncoder(&buf)
	encoder.SetIndent(indent)
	if err := encoder.Encode(v); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

