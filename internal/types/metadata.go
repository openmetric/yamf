package types

import (
	"fmt"
)

type Metadata map[string]interface{}

func (m Metadata) Get(key string) (interface{}, bool) {
	val, ok := m[key]
	return val, ok
}

func (m Metadata) GetString(key string) (string, bool) {
	if val, ok := m[key]; ok {
		return fmt.Sprintf("%v", val), ok
	} else {
		return "", ok
	}
}

func (m Metadata) Merge(other Metadata) {
	for k, v := range other {
		m[k] = v
	}
}

func (m Metadata) Copy() Metadata {
	new := make(Metadata)
	for k, v := range m {
		new[k] = v
	}
	return new
}
