package types

import (
	"encoding/json"
	"fmt"
	"github.com/HouzuoGuo/tiedot/db"
	"github.com/HouzuoGuo/tiedot/dberr"
	"github.com/fatih/structs"
)

// Rule defines a check and how to schedule check tasks.
type Rule struct {
	// `json` tag is for http api serialization, `structs` is for tiedot database serialization
	Type                   string                 `json:"type" structs:"type"`
	Check                  Check                  `json:"check" structs:"check,omitnested"`
	Metadata               map[string]interface{} `json:"metadata" structs:"metadata"`
	EventIdentifierPattern string                 `json:"event_identifier_pattern" structs:"event_identifier_pattern"`

	// schedule information
	Paused   bool     `json:"paused" structs:"paused"`
	Interval Duration `json:"interval" structs:"interval,string"`
	Timeout  Duration `json:"timeout" structs:"timeout,flatten"`

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

	if r.Timeout.Duration < r.Interval.Duration {
		return fmt.Errorf("Timeout must be less-equal than Interval")
	}

	if err := r.Check.Validate(); err != nil {
		return err
	}

	return nil
}

// Map wraps fatih/structs.Map, returns map[string]interface{} of the Rule struct
func (r *Rule) Map() map[string]interface{} {
	m := structs.Map(r)
	return m
}

// RuleDB wraps underlying database operation
type RuleDB struct {
	db  *db.DB
	col *db.Col
}

// NewRuleDB creates an RuleDB instance
func NewRuleDB(dbPath string) (*RuleDB, error) {
	var err error
	rdb := &RuleDB{}

	if rdb.db, err = db.OpenDB(dbPath); err != nil {
		return nil, err
	}

	if rdb.col = rdb.db.Use("Rules"); rdb.col == nil {
		_ = rdb.db.Create("Rules")
		rdb.col = rdb.db.Use("Rules")
	}

	return rdb, nil
}

// GetAll rules from db
func (rdb *RuleDB) GetAll() ([]*Rule, error) {
	if rdb.db == nil {
		return nil, fmt.Errorf("query on closed db")
	}

	rules := make([]*Rule, 0)

	rdb.col.ForEachDoc(func(id int, doc []byte) (moveOn bool) {
		rule := &Rule{}
		var err error

		if err = json.Unmarshal(doc, &rule); err != nil {
			// TODO log error
			return true
		}

		// ID is not stored in db
		rule.ID = id
		rules = append(rules, rule)
		return true
	})

	return rules, nil
}

// Get a rule by id
func (rdb *RuleDB) Get(id int) (*Rule, error) {
	if rdb.db == nil {
		return nil, fmt.Errorf("query on closed db")
	}

	rule := &Rule{}
	var doc map[string]interface{}
	var docB []byte
	var err error

	// TODO, avoid two phase Marshal+Unmarshal, consider using mitchellh/mapstructure
	if doc, err = rdb.col.Read(id); err != nil {
		if dberr.Type(err) == dberr.ErrorNoDoc {
			return nil, nil
		}
		return nil, err
	}

	if docB, err = json.Marshal(doc); err != nil {
		return nil, err
	}

	if err = json.Unmarshal(docB, rule); err != nil {
		return nil, err
	}

	// ID is not stored in db
	rule.ID = id
	return rule, nil
}

// Insert a rule to database, returns id if successfully inserted
func (rdb *RuleDB) Insert(rule *Rule) (int, error) {
	if rdb.db == nil {
		return 0, fmt.Errorf("query on closed db")
	}

	id, err := rdb.col.Insert(rule.Map())
	if err == nil {
		rule.ID = id
	}
	return id, err
}

// Update an existing rule in database
func (rdb *RuleDB) Update(id int, rule *Rule) error {
	if rdb.db == nil {
		return fmt.Errorf("query on closed db")
	}

	err := rdb.col.Update(id, rule.Map())
	if err == nil {
		rule.ID = id
	}
	return err
}

// Delete a rule by id
func (rdb *RuleDB) Delete(id int) error {
	if rdb.db == nil {
		return fmt.Errorf("delete on closed db")
	}

	return rdb.col.Delete(id)
}

// Close rule db
func (rdb *RuleDB) Close() error {
	err := rdb.db.Close()

	rdb.db = nil
	rdb.col = nil

	return err
}
