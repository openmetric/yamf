package types

import (
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

	// Event status: 0-OK, 1-Warning, 2-Critical, 3-Unknown
	Status int `json:"status"`
	// When the event was emitted
	Timestamp time.Time `json:"timestamp"`
	// extra description text
	Description string `json:"description"`
	// Event identifier
	Identifier string `json:"identifier"`

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

func (e *Event) SetResult(r Result) {
	e.Result = r
	e.Status = r.GetStatus()
	e.Description = r.GetDescription()
	MergeMap(r.GetMetadata(), e.Metadata)
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

// message of snmptrap
type SNMPTrapResult struct {
}
