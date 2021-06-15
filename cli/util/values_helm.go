package util

import (
	"io/ioutil"

	"github.com/pkg/errors"
	"sigs.k8s.io/yaml"
)

// ValuesFromFile retrieves value from file on given `path` structured as map.
func ValuesFromFile(path string) (map[string]interface{}, error) {
	var (
		values map[string]interface{}
	)
	armValYaml, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, errors.Wrapf(err,
			"missing ARM64 configuration values on path %s", path,
		)
	}

	if err = yaml.Unmarshal(armValYaml, &values); err != nil {
		return nil, errors.Wrapf(err,
			"failed to decode ARM64 configuration YAMl file on path %s",
			path,
		)
	}

	return values, nil
}
