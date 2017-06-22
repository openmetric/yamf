package utils

import (
	"gopkg.in/yaml.v2"
	"io/ioutil"
)

func UnmarshalYAMLFile(path string, holder interface{}) error {
	var err error
	var content []byte

	if content, err = ioutil.ReadFile(path); err != nil {
		return err
	}

	if err = yaml.Unmarshal(content, holder); err != nil {
		return err
	}

	return nil
}
