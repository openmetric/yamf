package main

import (
	"time"
)

// Task defines what to do
type Task struct {
	// Type of the task, currently only "graphite" is planned
	//   * graphite: performs a graphite render query and check the response data
	Type string

	// if ``Type`` is graphite, then the details is attaches as ``GraphiteTask``
	GraphiteTask GraphiteTask

	// When a executor fetches a task, it should check if current time is beyond
	// ``Expiration``, if the task has expired, there is no meaning to do this task
	// any more, the executor should skip this task.
	Expiration time.Time

	// The max time should an executor spend on executing this task. If any step
	// (e.g. querying graphite api) stucks too long, the executor should abort the task.
	Timeout time.Duration

	// Metadata passed from ``Rule`` and further passed on to ``Event``
	Metadata map[string]string

	// RuleID of the ``Rule`` which generated this Task, it is useless for the executor,
	// but can be passed on and ``Event``s' consumer may found it useful.
	RuleID int
}

// GraphiteTask defines a graphite query and compare expression
type GraphiteTask struct {
	// Used to form graphite render api queries,
	//   --> "?target={Query}&from={From}&until={Until}"
	Query string
	From  string
	Until string

	// Pattern used to extract metadata from api response.
	// MetaExtractPattern is a regular expression with named capture groups.
	// If a pattern is provided, ``target`` which do not match would be ignored,
	// not checks will be performed and thus no events would be yield.
	// If no pattern is provided (is empty string), then all ``target``s will be
	// further processed, but no metadata would be extracted.
	// Examples:
	//   pattern: "^(?P<resource_type>[^.]+)\.(?P<host>[^.]+)\.[^.]+\.user$"
	//   * "pm.server1.cpu.user" matches, with: resource_type=pm, host=server1
	//   * "pm.server2.cpu.user" matches, with: resource_type=pm, host=server2
	//   * "pm.server3.cpu.idle" does not match, and so ignored
	MetaExtractPattern string

	// Threshold of warning and critical, must be in the following forms:
	//   "> 1.0", ">= 1.0", "== 1.0", "<= 1.0", "< 1.0", "== nil", "!= nil"
	// The last value of a series is used as left operand. If the last value is
	// nil but expression is not nil related, Unknown is yield.
	// Evaluation order:
	//   * evaluate critical expression, next if not satisfied, or yield "critical"
	//   * evaluate warning expression, next if not satisfied, or yield "warning"
	//   * yield "ok"
	CriticalExpr string
	WarningExpr  string

	// Specify graphite api url, so we can query different graphite instances.
	// NOT to be implemented for first release.
	EndpointURL string
}

// Rule defines how to schedule tasks
type Rule struct {
	// The following fields are copied to generated ``Task`` as is
	Type         string
	GraphiteTask GraphiteTask
	Timeout      time.Duration

	// At which interval should tasks be generated
	Interval time.Duration

	// a rule can be paused, so that no tasks are generated
	Paused bool

	// Metadata to be passed on to ``Task``s and further ``Event``s
	Metadata map[string]string

	// ID of the rule
	ID int
}

func main() {
}
