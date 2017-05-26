package main

import (
	"encoding/json"
	"fmt"
	"github.com/HouzuoGuo/tiedot/db"
	"github.com/HouzuoGuo/tiedot/dberr"
	"github.com/fatih/structs"
	"math/rand"
	"time"
)

// Rule defines a check and how to schedule check tasks.
type Rule struct {
	Type  string          `json:"type" structs:"type"`
	Check CheckDefinition `json:"check" structs:"check,omitnested"` // check definition

	ID       int      `json:"id" structs:"-"`                     // database document id of the rule
	Paused   bool     `json:"paused" structs:"paused"`            // whether to schedule check tasks
	Interval Duration `json:"interval" structs:"interval,string"` // check interval
	Timeout  Duration `json:"timeout" structs:"timeout,flatten"`  // max execution time of a single check, should be smaller than interval

	Metadata map[string]interface{} `json:"metadata" structs:"metadata"`

	// used internally, stop signal
	stop chan struct{}
}

// NewRuleFromJSON unmarshals a json buffer into Rule object.
// If ignoreID is true, the returned Rule object will have ID=0
func NewRuleFromJSON(spec []byte, ignoreID bool) (*Rule, error) {
	var err error
	var holder = new(struct {
		Rule
		Check json.RawMessage `json:"check"`
	})

	if err = json.Unmarshal(spec, holder); err != nil {
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

	if ignoreID {
		rule.ID = 0
	}

	return rule, nil
}

// convert struct to a map[string]interface{} to be inserted into tiedot
func (rule *Rule) Map() map[string]interface{} {
	r := structs.Map(rule)
	return r
}

// Start scheduling check tasks.
func (rule *Rule) Start(publish func(*Task)) {
	rule.stop = make(chan struct{})

	go func() {
		// sleep a random time (between 0 and interval), so that checks can be distributes evenly.
		sleep := time.Duration(rand.Int63n(rule.Interval.Nanoseconds())) * time.Nanosecond
		time.Sleep(sleep)

		ticker := time.NewTicker(rule.Interval.Duration)
		for {
			select {
			case <-ticker.C:
				// TODO generate and publish a task
				task := &Task{
					RuleID: rule.ID,
				}
				publish(task)
			case <-rule.stop:
				ticker.Stop()
				rule.stop = nil
				return
			}
		}
	}()
}

// Stop scheduling check tasks.
func (rule *Rule) Stop() {
	if rule.stop != nil {
		close(rule.stop)
	}
}

type RuleDB struct {
	db  *db.DB
	col *db.Col
}

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

func (rdb *RuleDB) GetAllRules() ([]*Rule, error) {
	rules := make([]*Rule, 0)

	rdb.col.ForEachDoc(func(id int, doc []byte) (moveOn bool) {
		var rule *Rule
		var err error

		if rule, err = NewRuleFromJSON(doc, false); err != nil {
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

func (rdb *RuleDB) GetRule(id int) (*Rule, error) {
	var rule *Rule
	var doc map[string]interface{}
	var docB []byte
	var err error

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

func (rdb *RuleDB) InsertRule(rule *Rule) (int, error) {
	id, err := rdb.col.Insert(rule.Map())
	if err == nil {
		rule.ID = id
	}
	return id, err
}

func (rdb *RuleDB) UpdateRule(id int, rule *Rule) error {
	err := rdb.col.Update(id, rule.Map())
	if err == nil {
		rule.ID = id
	}
	return err
}

func (rdb *RuleDB) DeleteRule(id int) error {
	return rdb.col.Delete(id)
}

func (rdb *RuleDB) Close() error {
	err := rdb.db.Close()

	rdb.db = nil
	rdb.col = nil

	return err
}
