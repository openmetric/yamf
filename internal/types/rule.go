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

	Type  string          `json:"type" structs:"type"`
	Check CheckDefinition `json:"check" structs:"check,omitnested"` // check definition

	ID       int      `json:"id" structs:"-"`                     // database document id of the rule
	Paused   bool     `json:"paused" structs:"paused"`            // whether to schedule check tasks
	Interval Duration `json:"interval" structs:"interval,string"` // check interval
	Timeout  Duration `json:"timeout" structs:"timeout,flatten"`  // max execution time of a single check, should be smaller than interval

	Metadata map[string]interface{} `json:"metadata" structs:"metadata"`
}

// NewRuleFromJSON unmarshals a json buffer into Rule object.
// If resetID is true, the returned Rule object will have ID=0
func NewRuleFromJSON(data []byte, resetID bool) (*Rule, error) {
	var err error
	var holder = new(struct {
		Rule
		Check json.RawMessage `json:"check"`
	})

	if err = json.Unmarshal(data, holder); err != nil {
		return nil, err
	}

	rule := &holder.Rule

	switch rule.Type {
	case "graphite":
		rule.Check = new(GraphiteCheck)
	default:
		return nil, fmt.Errorf("Unsupported type: %s", rule.Type)
	}
	if err = json.Unmarshal(holder.Check, rule.Check); err != nil {
		return nil, err
	}

	if err = rule.Check.Validate(); err != nil {
		return nil, err
	}

	if rule.Interval.Duration <= 0 {
		return nil, fmt.Errorf("Invalid interval: %s", rule.Interval)
	}

	// timeout must be less equal than interval
	if rule.Timeout.Duration < rule.Interval.Duration {
		rule.Timeout = rule.Interval
	}

	if resetID {
		rule.ID = 0
	}

	return rule, nil
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
		var rule *Rule
		var err error

		if rule, err = NewRuleFromJSON(doc, true); err != nil {
			// TODO check error and move on
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

	var rule *Rule
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

	if rule, err = NewRuleFromJSON(docB, false); err != nil {
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
