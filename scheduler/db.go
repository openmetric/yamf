package main

import (
	"encoding/json"
	"github.com/HouzuoGuo/tiedot/db"
	"github.com/HouzuoGuo/tiedot/dberr"
)

// RuleStore is underlying db that stores rules
type RuleStore struct {
	db  *db.DB
	col *db.Col
}

// NewRuleStore creates a RuleStore instance
func NewRuleStore(dbPath string) (*RuleStore, error) {
	var err error
	var database *db.DB
	store := &RuleStore{}

	if database, err = db.OpenDB(dbPath); err != nil {
		return nil, err
	}
	store.db = database

	if store.col = database.Use("Rules"); store.col == nil {
		_ = database.Create("Rules")
		store.col = database.Use("Rules")
	}

	return store, nil
}

// LoadRules fetches all rules from database
func (store *RuleStore) LoadRules() ([]*Rule, error) {
	rules := make([]*Rule, 0)

	store.col.ForEachDoc(func(id int, doc []byte) (moveOn bool) {
		var rule *Rule
		var err error

		if rule, err = NewRuleFromJSON(doc); err != nil {
			// TODO check error
			return true
		}
		rule.ID = id
		rules = append(rules, rule)

		return true
	})

	return rules, nil
}

// LoadRule fetches the specified rule from database
func (store *RuleStore) LoadRule(id int) (*Rule, error) {
	var rule *Rule
	var doc map[string]interface{}
	var docB []byte
	var err error

	if doc, err = store.col.Read(id); err != nil {
		if dberr.Type(err) == dberr.ErrorNoDoc {
			return nil, nil
		}
		return nil, err
	}

	if docB, err = json.Marshal(doc); err != nil {
		return nil, err
	}

	if rule, err = NewRuleFromJSON(docB); err != nil {
		return nil, err
	}

	rule.ID = id

	return rule, nil
}

// SaveRule saves the rule to database
// If rule.ID is 0, the rule is inserted (created), otherwise, the rule is updated.
// On insert, rule.ID will be set afterwards.
func (store *RuleStore) SaveRule(rule *Rule) error {
	var err error
	var id int
	var docB []byte
	var doc map[string]interface{}

	// TODO, is there any better way to convert Rule struct into map[string]interface{}
	//       instead of Marshal()+Unmarshal()
	if docB, err = json.Marshal(rule); err != nil {
		return err
	}

	if err = json.Unmarshal(docB, &doc); err != nil {
		return err
	}

	if rule.ID == 0 {
		// insert
		if id, err = store.col.Insert(doc); err != nil {
			return err
		}
		rule.ID = id
	} else {
		// update
		if err = store.col.Update(rule.ID, doc); err != nil {
			return err
		}
	}

	return nil
}

func (store *RuleStore) DeleteRule(id int) error {
	return store.col.Delete(id)
}

func (store *RuleStore) Close() error {
	return store.db.Close()
}
