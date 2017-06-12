package types

import ()

const (
	OK       = 0
	Warning  = 1
	Critical = 2
	Unknown  = 3
)

type Event struct {
	Type        string `json:"type"`
	Source      string `json:"source"`
	Timestamp   Time
	Status      int      `json:"status"`
	Identifier  string   `json:"identifier"`
	Description string   `json:"description"`
	Metadata    Metadata `json:"metadata"`

	RuleID int    `json:"rule_id,omitempty"`
	Result Result `json:"result"`
}
