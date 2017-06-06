package types

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"sync"
)

// pattern used to parse threshold expression
var ThresholdExpressionRegexp = regexp.MustCompile(
	`^((?P<num_op>>|>=|==|<=|<|!=) *(?P<num_val>-?[0-9]+(\.[0-9]+)?)|(?P<nil_op>==|!=) *nil)$`,
)

// CheckDefinition is interface for check definitions
type CheckDefinition interface {
	Validate() error
}

// GraphiteCheck queries data from graphite and performs check on returned data
type GraphiteCheck struct {
	// Used to form graphite render api queries,
	//   --> "{GraphiteURL}/render/?target={Query}&from={From}&until={Until}"
	GraphiteURL string `json:"graphite_url"`
	Query       string `json:"query"`
	From        string `json:"from"`
	Until       string `json:"until"`

	// Pattern used to extract metadata from api response.
	// MetadataExtractPattern is a regular expression with named capture groups.
	// Examples:
	//   pattern: "^(?P<resource_type>[^.]+)\.(?P<host>[^.]+)\.[^.]+\.user$"
	//   * "pm.server1.cpu.user" matches, with: resource_type=pm, host=server1
	//   * "pm.server2.cpu.user" matches, with: resource_type=pm, host=server2
	//   * "pm.server3.cpu.idle" does not match, and so ignored
	MetadataExtractPattern string `json:"metadata_extract_pattern"`

	// Threshold of warning and critical, must be in the following forms:
	//   "> 1.0", ">= 1.0", "== 1.0", "<= 1.0", "< 1.0", "!= 1.0", "== nil", "!= nil"
	// The last value of a series is used as left operand. If the last value is
	// nil but expression is not nil related, Unknown is yield.
	// Evaluation order:
	//  critical -> warning -> ok
	CriticalExpression ThresholdExpression `json:"critical_expression"`
	WarningExpression  ThresholdExpression `json:"warning_expression"`

	// Usually, we use the last value in query result to compare threshold, however,
	// sometimes, due to time drift or carbon's cache, the last value is null.
	// In this case, we can find the last non null value to compare threshold, but
	// it's apparently no sense if there are too many null values. If there are more then
	// 'MaxNullPoints' null values in the end, the value will be considered as null.
	MaxNullPoints int `json:"max_null_points"`
}

// Validate the definition, return error description if any. Some values will be
// set to default if not provided.
func (c *GraphiteCheck) Validate() error {
	var err error

	if c.Query == "" {
		return fmt.Errorf("must provide `query` for graphite check")
	}

	if c.MetadataExtractPattern != "" {
		if _, err = RegexpCompile(c.MetadataExtractPattern); err != nil {
			return fmt.Errorf("failed to compile `metadata_extract_pattern` with error: %s", err)
		}
	}

	if c.MaxNullPoints < 0 {
		return fmt.Errorf("`allowed_null_points` must be great equal than 0")
	}

	return nil
}

type ThresholdExpression struct {
	str         string
	NumberOp    string
	NumberValue float64
	NullOp      string
}

var thresholdExpressionCache = struct {
	cache map[string]*ThresholdExpression
	sync.RWMutex
}{
	cache: make(map[string]*ThresholdExpression),
}

func NewThresholdExpression(str string) (*ThresholdExpression, error) {
	thresholdExpressionCache.Lock()
	defer thresholdExpressionCache.Unlock()

	if e, ok := thresholdExpressionCache.cache[str]; ok {
		return e, nil
	}

	if !ThresholdExpressionRegexp.MatchString(str) {
		return nil, fmt.Errorf("Invalid expression: %s", str)
	}

	e := &ThresholdExpression{
		str: str,
	}

	matches := ThresholdExpressionRegexp.FindStringSubmatch(str)
	names := ThresholdExpressionRegexp.SubexpNames()
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

	thresholdExpressionCache.cache[str] = e
	return e, nil
}

func (t *ThresholdExpression) IsNulllComparer() bool {
	return t.NullOp != ""
}

func (t *ThresholdExpression) Evaluate(value float64, absent bool) (result bool, unknown bool) {
	if !t.IsNulllComparer() {
		// if it's a number comparer, but data is null, the expression evaluate to false
		if absent {
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
			return absent, false
		case "!=":
			return !absent, false
		}
	}
	return false, true
}

func (e *ThresholdExpression) MarshalJSON() ([]byte, error) {
	return json.Marshal(e.str)
}

func (e *ThresholdExpression) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	if tmp, err := NewThresholdExpression(s); err != nil {
		return err
	} else {
		e.str = tmp.str
		e.NumberOp = tmp.NumberOp
		e.NumberValue = tmp.NumberValue
		e.NullOp = tmp.NullOp
		return nil
	}
}
