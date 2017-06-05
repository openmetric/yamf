package types

import (
	"fmt"
	"regexp"
	"strconv"
)

// CheckDefinition is interface for check definitions
type CheckDefinition interface {
	Validate() error
}

var ThresholdExprRegexp = regexp.MustCompile(`^((?P<num_op>>|>=|==|<=|<|!=) *(?P<num_val>-?[0-9]+(\.[0-9]+)?)|(?P<nil_op>==|!=) *nil)$`)

// GraphiteCheck queries data from graphite and performs check on returned data
type GraphiteCheck struct {
	// Used to form graphite render api queries,
	//   --> "?target={Query}&from={From}&until={Until}"
	Query string `json:"query"`
	From  string `json:"from"`
	Until string `json:"until"`

	// Pattern used to extract metadata from api response.
	// MetaExtractPattern is a regular expression with named capture groups.
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
	//  critical -> warning -> ok
	CriticalExpr string `json:"critical_expr"`
	WarningExpr  string `json:"warning_expr"`

	// Usually, we use the last value in query result to compare threshold, however,
	// sometimes, due to time drift or carbon's cache, the last value is null.
	// In this case, we can find the last non null value to compare threshold, but
	// it's apparently no sense if there are too many null values. If there are more then
	// 'AllowedNullPoints' null values in the end, the value will be considered as null.
	AllowedNullPoints int `json:"allowed_null_points"`

	// Specify graphite api url, so we can query different graphite instances.
	GraphiteURL string `json:"graphite_url"`
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

	if c.CriticalExpr != "" && !ThresholdExprRegexp.MatchString(c.CriticalExpr) {
		return fmt.Errorf("Invalid `critical_expr`")
	}
	if c.WarningExpr != "" && !ThresholdExprRegexp.MatchString(c.WarningExpr) {
		return fmt.Errorf("Invalid `warning_expr`")
	}

	if c.CriticalExpr == "" && c.WarningExpr == "" {
		return fmt.Errorf("Must specify at least one threshold expression")
	}

	if c.AllowedNullPoints < 0 {
		return fmt.Errorf("`allowed_null_points` must be great equal than 0")
	}

	return nil
}

type ThresholdExpr struct {
	NumberOp    string
	NumberValue float64
	NullOp      string
}

var thresholdExprCache map[string]*ThresholdExpr = make(map[string]*ThresholdExpr)

func NewThresholdExpr(expr string) *ThresholdExpr {
	if e, ok := thresholdExprCache[expr]; ok {
		return e
	}

	e := &ThresholdExpr{}

	matches := ThresholdExprRegexp.FindStringSubmatch(expr)
	names := ThresholdExprRegexp.SubexpNames()
	for i, match := range matches {
		switch names[i] {
		case "num_op":
			e.NumberOp = match
		case "num_val":
			e.NumberValue, _ = strconv.ParseFloat(match, 64)
		case "nil_op":
			e.NullOp = match
		}
	}

	thresholdExprCache[expr] = e
	return e
}

func (t *ThresholdExpr) IsNulllComparer() bool {
	return t.NullOp != ""
}

func (t *ThresholdExpr) Evaluate(value float64, isNull bool) (result bool, unknown bool) {
	if !t.IsNulllComparer() {
		// if it's a number comparer, but data is null, the expression evaluate to false
		if isNull {
			return false, true
		}
		switch t.NumberOp {
		case ">":
			return value > t.NumberValue, false
		case ">=":
			return value >= t.NumberValue, false
		case "==":
			return value == t.NumberValue, false
		case "!=":
			return value != t.NumberValue, false
		case "<=":
			return value <= t.NumberValue, false
		case "<":
			return value < t.NumberValue, false
		}
	} else {
		switch t.NullOp {
		case "==":
			return isNull, false
		case "!=":
			return !isNull, false
		}
	}
	return false, true
}
