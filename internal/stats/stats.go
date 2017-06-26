package stats

import (
	"github.com/openmetric/graphite-client"
	"os"
	"reflect"
	"strings"
	"sync/atomic"
	"time"
)

type Config struct {
	Interval time.Duration `yaml:"interval"`
	URL      string        `yaml:"url"`
	Prefix   string        `yaml:"prefix"`
	Enabled  bool          `yaml:"enabled"`
}

func (c *Config) UnmarshalYAML(unmarshal func(interface{}) error) error {
	type Alias Config
	aux := (*Alias)(c)

	if err := unmarshal(&aux); err != nil {
		return err
	}

	hostname, _ := os.Hostname()
	hostname = strings.Replace(hostname, ".", "-", -1)
	c.Prefix = strings.Replace(c.Prefix, "{host}", hostname, -1)

	return nil
}

func NewConfig() *Config {
	return &Config{
		Interval: 10 * time.Second,
		URL:      "tcp://localhost:2003",
		Prefix:   "yamf.{host}.",
		Enabled:  true,
	}
}

// Counter type for counter type metrics
type Counter struct {
	value uint64
}

// Add n to the counter
func (c *Counter) Add(n uint64) {
	atomic.AddUint64(&c.value, n)
}

// Inc the counter by 1
func (c *Counter) Inc() {
	atomic.AddUint64(&c.value, 1)
}

// Load counter's value
func (c *Counter) Load() uint64 {
	return atomic.LoadUint64(&c.value)
}

// Gauge type for gauge type metrics
type Gauge struct {
	value int64
}

// Set value of gauge
func (g *Gauge) Set(n int64) {
	atomic.StoreInt64(&g.value, n)
}

// Load gauge's value
func (g *Gauge) Load() int64 {
	return atomic.LoadInt64(&g.value)
}

func (g *Gauge) Add(n int64) {
	atomic.AddInt64(&g.value, n)
}

func (g *Gauge) Inc() {
	atomic.AddInt64(&g.value, 1)
}

func (g *Gauge) Dec() {
	atomic.AddInt64(&g.value, -1)
}

func ToGraphiteMetric(s interface{}, prefix string) []*graphite.Metric {
	val := reflect.ValueOf(s)
	typ := val.Type()
	numFields := val.NumField()
	var results []*graphite.Metric
	timestamp := time.Now().Unix()

	for i := 0; i < numFields; i++ {
		//if val.Field(i).Type().Kind() == reflect.Struct {
		//	prefix = typ.Field(i).Tag.Get("stats")
		//	subResults := ToGraphiteMetric(val.Field(i).Interface(), prefix)
		//	results = append(results, subResults...)
		//	continue
		//}
		switch value := val.Field(i).Interface().(type) {
		case Counter:
			name := typ.Field(i).Tag.Get("stats")
			if prefix != "" {
				name = prefix + "." + name
			}
			results = append(results, &graphite.Metric{
				Name:      name,
				Value:     value.Load(),
				Timestamp: timestamp,
			})
		case Gauge:
			name := typ.Field(i).Tag.Get("stats")
			if prefix != "" {
				name = prefix + "." + name
			}
			results = append(results, &graphite.Metric{
				Name:      name,
				Value:     value.Load(),
				Timestamp: timestamp,
			})
		case interface{}:
			prefix = typ.Field(i).Tag.Get("stats")
			subResults := ToGraphiteMetric(val.Field(i).Interface(), prefix)
			results = append(results, subResults...)
		}
	}
	return results
}
