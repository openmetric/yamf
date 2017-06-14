package types

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// Task is a scheduled check task.
type Task struct {
	// fields copied from rule as is
	Type                   string              `json:"type"`
	Check                  Check               `json:"check"`
	Metadata               Metadata            `json:"metadata"`
	EventIdentifierPattern *IdentifierTemplate `json:"event_identifier_pattern"`

	// execution instructions
	Schedule   Time `json:"schedule"`   // when the task was scheduled (emitted from scheduler)
	Deadline   Time `json:"deadline"`   // how long should the task execution take at most
	Expiration Time `json:"expiration"` // if now is beyond expiration, the task should not be executed

	RuleID int `json:"rule_id"` // the rule from which this task was generated
}

func NewTaskFromRule(r *Rule) *Task {
	now := time.Now()
	task := &Task{
		RuleID: r.ID,

		Type:                   r.Type,
		Check:                  r.Check,
		Metadata:               r.Metadata,
		EventIdentifierPattern: NewIdentifierTemplate(r.EventIdentifierPattern),

		Schedule:   FromTime(now),
		Deadline:   FromTime(now.Add(r.Timeout.Duration)),
		Expiration: FromTime(now.Add(r.Interval.Duration)),
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

type IdentifierTemplate struct {
	pattern  string
	subNames map[string]bool
}

var identifierTemplateCache = NewGenericCache(
	func(pattern interface{}) (interface{}, error) {
		re := RegexpMustCompile("{([^}]+)}")
		matches := re.FindAllStringSubmatch(pattern.(string), -1)
		subNames := make(map[string]bool)

		for _, match := range matches {
			subNames[match[1]] = true
		}

		t := &IdentifierTemplate{
			pattern:  pattern.(string),
			subNames: subNames,
		}

		return t, nil
	},
)

func NewIdentifierTemplate(pattern string) *IdentifierTemplate {
	t, _ := identifierTemplateCache.GetOrCreate(pattern)
	return t.(*IdentifierTemplate)
}

func (t *IdentifierTemplate) Parse(metadata Metadata) (string, error) {
	// TODO optomize
	// TODO check for possible errors, e.g. key does not exist in metadata
	var target = t.pattern

	for subname, _ := range t.subNames {
		var val string
		if v, ok := metadata[subname]; ok {
			val = fmt.Sprintf("%v", v)
			target = strings.Replace(target, "{"+subname+"}", val, -1)
		}
	}
	return target, nil
}

func (t *IdentifierTemplate) MarshalJSON() ([]byte, error) {
	return json.Marshal(t.pattern)
}

func (t *IdentifierTemplate) UnmarshalJSON(data []byte) error {
	var pattern string
	if err := json.Unmarshal(data, &pattern); err != nil {
		return err
	}

	tmp := NewIdentifierTemplate(pattern)
	t.pattern = tmp.pattern
	t.subNames = tmp.subNames
	return nil
}
