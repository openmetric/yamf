package types

import (
	"encoding/json"
	"fmt"
	"time"
)

// Task is a scheduled check task.
type Task struct {
	// fields copied from rule as is
	Type                   string                 `json:"type"`
	Check                  Check                  `json:"check"`
	Metadata               map[string]interface{} `json:"metadata"`
	EventIdentifierPattern string                 `json:"event_identifier_pattern"`

	// execution instructions
	Schedule   time.Time `json:"schedule"`   // when the task was scheduled (emitted from scheduler)
	Timeout    Duration  `json:"timeout"`    // how long should the task execution take at most
	Expiration time.Time `json:"expiration"` // if now is beyond expiration, the task should not be executed

	RuleID int `json:"rule_id"` // the rule from which this task was generated
}

func NewTaskFromRule(r *Rule) *Task {
	now := time.Now()
	task := &Task{
		RuleID: r.ID,

		Type:                   r.Type,
		Check:                  r.Check,
		Metadata:               r.Metadata,
		EventIdentifierPattern: r.EventIdentifierPattern,

		Schedule:   now,
		Timeout:    r.Timeout,
		Expiration: now.Add(r.Timeout.Duration),
	}
	return task
}

func (t *Task) UnmarshalJSON(data []byte) error {
	var err error

	type Alias Task
	aux := &struct {
		*Alias
		Check json.RawMessage `json:"check"`
	}{
		Alias: (*Alias)(t),
	}

	if err = json.Unmarshal(data, aux); err != nil {
		return err
	}

	switch t.Type {
	case "graphite":
		t.Check = new(GraphiteCheck)
	default:
		return fmt.Errorf("Unsupported type: %s", t.Type)
	}
	if err = json.Unmarshal(aux.Check, t.Check); err != nil {
		return err
	}

	return nil
}
