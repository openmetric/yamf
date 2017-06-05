package types

import (
	"time"
)

type Event struct {
	// source of this event, "rule", "snmptrap", "push" ...
	Source string `json:"source"`

	// event status: 0-OK, 1-Warning, 2-Critical, 3-Unknown
	Status      int    `json:"status"`
	Description string `json:"description"`

	// if the event is emitted by a rule, these fields are copied from rule
	Type   string `json:"type,omitempty"`
	RuleID int    `json:"rule_id,omitempty"`

	// merge of metadata attached with rule and computed data
	Metadata map[string]interface{} `json:"metadata"`

	// if the event is emitted by a rule of graphite check, GraphiteCheckResult holds the check result
	Result Result `json:"result"`
}

type Result interface{}

// result of graphite check
type GraphiteCheckResult struct {
	ScheduleTime  time.Time `json:"schedule_time"`  // when the task was scheduled
	ExecutionTime time.Time `json:"execution_time"` // when the task was executed

	MetricTime  time.Time `json:"metric_time"`  // the timestamp of metric used to compare
	MetricValue float64   `json:"metric_value"` // the metric value used to compare
}

// message of snmptrap
type SNMPTrapResult struct {
}
