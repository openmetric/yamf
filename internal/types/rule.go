package types

import (
	"encoding/json"
	"fmt"
	"github.com/fatih/structs"
)

// Rule defines a check and how to schedule check tasks.
type Rule struct {
	// `json` tag is for http api serialization, `structs` is for tiedot database serialization
	Type                   string   `json:"type" structs:"type"`
	Check                  Check    `json:"check" structs:"check,omitnested"`
	Metadata               Metadata `json:"metadata" structs:"metadata"`
	EventIdentifierPattern string   `json:"event_identifier_pattern" structs:"event_identifier_pattern"`

	// schedule information
	Paused   bool     `json:"paused" structs:"paused"`
	Interval Duration `json:"interval" structs:"interval,string"`
	Timeout  Duration `json:"timeout" structs:"timeout,string"`

	// database id
	ID int `json:"id" structs:"-"`
}

func (r *Rule) UnmarshalJSON(data []byte) error {
	var err error

	type Alias Rule
	aux := &struct {
		*Alias
		Check json.RawMessage `json:"check"`
	}{
		Alias: (*Alias)(r),
	}
	if err = json.Unmarshal(data, aux); err != nil {
		return err
	}

	switch r.Type {
	case "graphite":
		r.Check = new(GraphiteCheck)
	default:
		return fmt.Errorf("Unsupported type: %s", r.Type)
	}
	if err = json.Unmarshal(aux.Check, r.Check); err != nil {
		return err
	}

	return nil
}

func (r *Rule) Validate() error {
	if r.Interval.Duration <= 0 {
		return fmt.Errorf("Invalid interval: %s", r.Interval)
	}

	if r.Timeout.Duration > r.Interval.Duration {
		return fmt.Errorf("Timeout must be less-equal than Interval")
	}

	if err := r.Check.Validate(); err != nil {
		return err
	}

	return nil
}

func (r *Rule) MarshalMap() map[string]interface{} {
	return structs.Map(r)
}

func (r *Rule) UnmarshalMap(data map[string]interface{}) error {
	var jsonData []byte
	var err error
	if jsonData, err = json.Marshal(data); err != nil {
		return err
	}
	if err = json.Unmarshal(jsonData, r); err != nil {
		return err
	}
	return nil
}
