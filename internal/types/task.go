package types

import (
	"time"
)

// Task is a scheduled check task.
type Task struct {
	// type of the task graphite, elasticsearch, ...
	Type string

	Schedule   time.Time // when the task was scheduled (emitted from scheduler)
	Timeout    Duration  // how long should the task execution take at most
	Expiration time.Time // if now is beyond expiration, the task should not be executed

	// hold data passed from rule, and attach more data after execution if any
	Metadata map[string]interface{}

	RuleID int // the rule from which this task was generated

	Check CheckDefinition // check definition
}
