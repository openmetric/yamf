package types

import (
	"encoding/json"
	"sync"
	"time"
)

// Duration is a wrapper around time.Duration which implements json.Unmarshaler and json.Marshaler.
// It marshals and unmarshals the duration as a string in the format accepted by time.ParseDuration and returned by time.Duration.String.
type Duration struct {
	time.Duration
}

// FromDuration is a convenience factory to create a Duration instance from the given time.Duration value.
func FromDuration(d time.Duration) Duration {
	return Duration{d}
}

// MarshalJSON implements the json.Marshaler interface.
// The duration is a quoted-string in the format accepted by time.ParseDuration and returned by time.Duration.String
func (d Duration) MarshalJSON() ([]byte, error) {
	return []byte(`"` + d.String() + `"`), nil
}

// UnmarshalJSON implements the json.Unmarshaler interface.
// The duration is expected to be a quoted-string of a duration in the format accepted by time.ParseDuration.
func (d *Duration) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}

	tmp, err := time.ParseDuration(s)
	if err != nil {
		return err
	}

	d.Duration = tmp

	return nil
}

func CopyMap(src map[string]interface{}) map[string]interface{} {
	dst := make(map[string]interface{})
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func MergeMap(dst, src map[string]interface{}) {
	for k, v := range src {
		dst[k] = v
	}
}

type GenericCache struct {
	cache  map[interface{}]interface{}
	create CacheObjectCreateFunc
	sync.RWMutex
}

type CacheObjectCreateFunc func(interface{}) (interface{}, error)

func NewGenericCache(create CacheObjectCreateFunc) *GenericCache {
	cache := &GenericCache{
		cache:  make(map[interface{}]interface{}),
		create: create,
	}
	return cache
}

func (c *GenericCache) GetOrCreate(k interface{}) (v interface{}, err error) {
	c.Lock()
	defer c.Unlock()

	if v, ok := c.cache[k]; ok {
		return v, nil
	}

	if v, err = c.create(k); err == nil {
		c.cache[k] = v
	}
	return v, err
}
