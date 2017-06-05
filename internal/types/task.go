package types

import (
	"encoding/json"
	"fmt"
	"time"
)

// Task is a scheduled check task.
type Task struct {
	// type of the task graphite, elasticsearch, ...
	Type string `json:"type"`

	Schedule   time.Time `json:"schedule"`   // when the task was scheduled (emitted from scheduler)
	Timeout    Duration  `json:"duration"`   // how long should the task execution take at most
	Expiration time.Time `json:"expiration"` // if now is beyond expiration, the task should not be executed

	// hold data passed from rule, and attach more data after execution if any
	Metadata map[string]interface{} `json:"metadata"`

	RuleID int `json:"rule_id"` // the rule from which this task was generated

	Check CheckDefinition `json:"check"` // check definition
}

func NewTaskFromRule(rule *Rule) *Task {
	now := time.Now()
	task := &Task{
		Type:       rule.Type,
		Check:      rule.Check,
		Schedule:   now,
		Timeout:    rule.Timeout,
		Expiration: now.Add(rule.Timeout.Duration),
		Metadata:   rule.Metadata,
		RuleID:     rule.ID,
	}
	return task
}

// NewTaskFromJSON unmarshals a json buffer into Task object.
func NewTaskFromJSON(data []byte) (*Task, error) {
	var err error
	var holder = new(struct {
		Task
		Check json.RawMessage `json:"check"`
	})

	if err = json.Unmarshal(data, holder); err != nil {
		return nil, err
	}

	task := &holder.Task

	switch task.Type {
	case "graphite":
		task.Check = new(GraphiteCheck)
	default:
		return nil, fmt.Errorf("Unsupported type: %s", task.Type)
	}
	if err = json.Unmarshal(holder.Check, task.Check); err != nil {
		return nil, err
	}

	if err = task.Check.Validate(); err != nil {
		return nil, err
	}

	return task, nil
}
