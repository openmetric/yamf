package types

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"
)

const (
	OK       = 0
	Warning  = 1
	Critical = 2
	Unknown  = 3
)

type Event struct {
	// source of this event, "rule", "snmptrap", "push" ...
	Source string `json:"source"`

	// When the event was emitted
	Timestamp time.Time `json:"timestamp"`

	IdentifierTemplate *IdentifierTemplate `json:"-"`

	// if the event is emitted by a rule, these fields are copied from rule
	Type   string `json:"type,omitempty"`
	RuleID int    `json:"rule_id,omitempty"`

	// merge of metadata attached with rule and computed data
	Metadata map[string]interface{} `json:"metadata"`

	// if the event is emitted by a rule of graphite check, GraphiteResult holds the check result
	Result Result `json:"result"`
}

func NewEvent(source string) *Event {
	event := &Event{
		Source: source,
		// Status is set afterwards
	}
	return event
}

func (e *Event) MarshalJSON() ([]byte, error) {
	type Alias Event
	aux := &struct {
		*Alias
		Status      int    `json:"status"`
		Description string `json:"description"`
		Identifier  string `json:"identifier"`
	}{
		Alias:       (*Alias)(e),
		Status:      e.Result.GetStatus(),
		Description: e.Result.GetDescription(),
	}

	if e.IdentifierTemplate != nil {
		var err error
		if aux.Identifier, err = e.IdentifierTemplate.Parse(e.Metadata); err != nil {
			return nil, err
		}
	}

	return json.Marshal(aux)
}

func (e *Event) SetResult(r Result) {
	MergeMap(e.Metadata, r.GetMetadata())
	e.Result = r
}

type IdentifierTemplate struct {
	pattern string
	subs    map[string]bool
}

var subsCache = struct {
	cache map[string]map[string]bool
	sync.RWMutex
}{
	cache: make(map[string]map[string]bool),
}

func NewIdentifierTemplate(pattern string) *IdentifierTemplate {
	subsCache.Lock()
	defer subsCache.Unlock()

	if subs, ok := subsCache.cache[pattern]; ok {
		return &IdentifierTemplate{
			pattern: pattern,
			subs:    subs,
		}
	}

	re := RegexpMustCompile("{([^}]+)}")
	matches := re.FindAllStringSubmatch(pattern, -1)
	subs := make(map[string]bool)

	for _, match := range matches {
		subs[match[1]] = true
	}

	subsCache.cache[pattern] = subs

	return &IdentifierTemplate{
		pattern: pattern,
		subs:    subs,
	}
}

func (i *IdentifierTemplate) Parse(metadata map[string]interface{}) (string, error) {
	// TODO optomize
	// TODO check for possible errors, e.g. key does not exist in metadata
	var target = i.pattern
	for sub, _ := range i.subs {
		var val string
		if v, ok := metadata[sub]; ok {
			val = fmt.Sprintf("%v", v)
			target = strings.Replace(target, "{"+sub+"}", val, -1)
		}
	}
	return target, nil
}

type Result interface {
	GetStatus() int
	GetMetadata() map[string]interface{}
	GetDescription() string
}

// result of graphite check
type GraphiteResult struct {
	Status          int                    `json:"status"`
	MetricName      string                 `json:"metric_name"`
	MetricTimestamp time.Time              `json:"metric_timestamp"` // the timestamp of metric used to compare
	MetricValue     float64                `json:"metric_value"`     // the metric value used to compare
	Metadata        map[string]interface{} `json:"metadata"`         // metadata extracted

	ExecutionTimestamp time.Time `json:"execution_timestamp"` // when the check was executed
}

func (r *GraphiteResult) GetStatus() int {
	return r.Status
}

func (r *GraphiteResult) GetMetadata() map[string]interface{} {
	return r.Metadata
}

func (r *GraphiteResult) GetDescription() string {
	return ""
}
