package helm

import (
	"fmt"
	"io/ioutil"

	"sigs.k8s.io/yaml"
)

// ValuesFromFile retrieves value from file on given `path` structured as map.
func ValuesFromFile(path string) (map[string]interface{}, error) {
	var (
		values map[string]interface{}
	)

	armValYaml, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("missing configuration values on path %s: %w", path, err)
	}

	if err = yaml.Unmarshal(armValYaml, &values); err != nil {
		return nil, fmt.Errorf("failed to decode configuration YAMl file on path %s: %w", path, err)
	}

	return values, nil
}
