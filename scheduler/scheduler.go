package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"regexp"
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
	Metadata map[string]interface{}

	// RuleID of the ``Rule`` which generated this Task, it is useless for the executor,
	// but can be passed on and ``Event``s' consumer may found it useful.
	RuleID int

	// When this task was published, started, finished
	PublishTime time.Time
	StartTime   time.Time
	FinishTime  time.Time
}

// GraphiteTask defines a graphite query and compare expression
type GraphiteTask struct {
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

// Rule defines how to schedule tasks
type Rule struct {
	// The following fields are copied to generated ``Task`` as is
	Type          string        `json:"type"`
	GraphiteTask  GraphiteTask  `json:"graphite_task"`
	TimeoutString string        `json:"timeout"`
	Timeout       time.Duration `json:"-"`

	// At which interval should tasks be generated
	// Since json.Unmarshal() does not support representation like "10s", we convert manually
	IntervalString string        `json:"interval"`
	Interval       time.Duration `json:"-"`

	// a rule can be paused, so that no tasks are generated
	Paused bool `json:"paused"`

	// Metadata to be passed on to ``Task``s and further ``Event``s
	Metadata map[string]interface{} `json:"metadata"`

	// ID of the rule
	ID int `json:"id"`

	// used internally, used to stop a rule
	stop chan struct{}
}

// StartScheduling starts a goroutine to generate tasks periodically
func (rule *Rule) StartScheduling() {
	rule.stop = make(chan struct{})

	go func() {
		// sleep a random time (between 0 and interval), so that checks can be distributed evenly.
		time.Sleep(time.Duration(rand.Int63n(rule.Interval.Nanoseconds())) * time.Nanosecond)

		ticker := time.NewTicker(rule.Interval)
		for {
			select {
			case <-ticker.C:
				// Generate and publish a Task
				task := Task{
					Type:         rule.Type,
					GraphiteTask: rule.GraphiteTask,
					Expiration:   time.Now().Add(rule.Interval),
					Timeout:      rule.Timeout,
					Metadata:     rule.Metadata,
					RuleID:       rule.ID,
					PublishTime:  time.Now(),
				}
				PublishTask(&task)
			case <-rule.stop:
				ticker.Stop()
				rule.stop = nil
				return
			}
		}
	}()
}

// StopScheduling stops the goroutine which is generating tasks
func (rule *Rule) StopScheduling() {
	if rule.stop != nil {
		close(rule.stop)
	}
}

// NewRuleFromJSON creates a Rule object by decoding spec as json
func NewRuleFromJSON(spec []byte) (*Rule, error) {
	var err error

	rule := &Rule{}

	if err = json.Unmarshal(spec, rule); err != nil {
		return nil, err
	}

	// check if all fields meets requirement

	// ensure rule.ID is not set
	if rule.ID != 0 {
		return nil, errors.New("`id` must NOT be set")
	}

	// ensure rule.Type is set
	if rule.Type == "" {
		return nil, errors.New("`type` must be set")
	}

	// ensure rule.Interval is set
	if rule.IntervalString == "" {
		return nil, errors.New("`interval` must be set")
	}
	if rule.Interval, err = time.ParseDuration(rule.IntervalString); err != nil {
		return nil, err
	}

	// set proper rule.Timeout
	if rule.TimeoutString == "" {
		rule.Timeout = rule.Interval
	}
	if rule.Timeout, err = time.ParseDuration(rule.TimeoutString); err != nil {
		return nil, err
	}
	if rule.Timeout > rule.Interval {
		rule.Timeout = rule.Interval
	}

	if rule.Type == "graphite" {
		if err = checkGraphiteTask(&rule.GraphiteTask); err != nil {
			return nil, err
		}
	}

	return rule, nil
}

// check if the provided graphte task is valid, and set proper values
func checkGraphiteTask(task *GraphiteTask) error {
	var err error

	if task.Query == "" {
		return errors.New("`query` must be set")
	}

	if task.From == "" {
		task.From = "-5min"
	}
	if task.Until == "" {
		task.Until = "now"
	}

	if task.MetaExtractPattern != "" {
		if _, err = regexp.Compile(task.MetaExtractPattern); err != nil {
			return err
		}
	}

	compareExprRegexp, _ := regexp.Compile(`^((>|>=|==|<=|<|!=) *-?[0-9]+(\.[0-9]+)?|(==|!=) *nil)$`)
	if !compareExprRegexp.MatchString(task.CriticalExpr) {
		return errors.New("invalid `critical_expr`")
	}
	if !compareExprRegexp.MatchString(task.WarningExpr) {
		return errors.New("invalid `warning_expr`")
	}

	return nil
}

// PublishTask publishes the task
func PublishTask(task *Task) {
	// TODO implement me
	fmt.Printf("%#v\n", task)
}

func main() {
	ruleSpec := []byte(`
{
	"type": "graphite",
	"timeout": "2m",
	"interval": "2s",
	"metadata": { "rule-id": "test.check" },
	"graphite_task": {
		"query": "pm.*.agg.cpu.percent",
		"from": "-5min",
		"until": "now",
		"meta_extract_pattern": "^$",
		"critical_expr": "> -1.0",
		"warning_expr": "> 0.8"
	},

	"dummy": "dummy"
}
`)
	rule, err := NewRuleFromJSON(ruleSpec)
	if err != nil {
		fmt.Println(err)
	} else {
		fmt.Printf("%#v\n", rule)
	}

	rule.StartScheduling()

	time.Sleep(10 * time.Second)

	fmt.Println("stopping the rule")
	rule.StopScheduling()
	fmt.Println("rule stopped")

	time.Sleep(10 * time.Second)
}
