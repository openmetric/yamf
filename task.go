package main

import (
	"fmt"
	"regexp"
	"time"
)

// CheckDefinition is interface for check definitions
type CheckDefinition interface {
	Validate() error
}

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

// GraphiteCheck queries data from graphite and performs check on returned data
type GraphiteCheck struct {
	// Used to form graphite render api queries,
	//   --> "?target={Query}&from={From}&until={Until}"
	Query string `json:"query"`
	From  string `json:"from"`
	Until string `json:"until"`

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
	MetaExtractPattern string `json:"meta_extract_pattern"`

	// Threshold of warning and critical, must be in the following forms:
	//   "> 1.0", ">= 1.0", "== 1.0", "<= 1.0", "< 1.0", "!= 1.0", "== nil", "!= nil"
	// The last value of a series is used as left operand. If the last value is
	// nil but expression is not nil related, Unknown is yield.
	// Evaluation order:
	//   * evaluate critical expression, next if not satisfied, or yield "critical"
	//   * evaluate warning expression, next if not satisfied, or yield "warning"
	//   * yield "ok"
	CriticalExpr string `json:"critical_expr"`
	WarningExpr  string `json:"warning_expr"`

	// Specify graphite api url, so we can query different graphite instances.
	// NOT to be implemented for first release.
	EndpointURL string `json:"endpoint_url"`
}

// Validate the definition, return error description if any. Some values will be
// set to default if not provided.
func (c *GraphiteCheck) Validate() error {
	var err error

	if c.Query == "" {
		return fmt.Errorf("must provide `query` for graphite check")
	}

	if c.MetaExtractPattern != "" {
		if _, err = regexp.Compile(c.MetaExtractPattern); err != nil {
			return fmt.Errorf("failed to compile `meta_extract_pattern` with error: %s", err)
		}
	}

	thresholdExprRegexp, _ := regexp.Compile(`^((>|>=|==|<=|<|!=) *-?[0-9]+(\.[0-9]+)?|(==|!=) *nil)$`)
	if !thresholdExprRegexp.MatchString(c.CriticalExpr) {
		return fmt.Errorf("Invalid `critical_expr`")
	}
	if !thresholdExprRegexp.MatchString(c.WarningExpr) {
		return fmt.Errorf("Invalid `warning_expr`")
	}

	return nil
}
