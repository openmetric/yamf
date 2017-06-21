package ruledb

import (
	"encoding/json"
	"fmt"
	"github.com/HouzuoGuo/tiedot/db"
	"github.com/openmetric/yamf/internal/types"
)

type RuleDB struct {
	db  *db.DB
	col *db.Col
}

func NewRuleDB(dbPath, dbCollection string) (*RuleDB, error) {
	rdb := &RuleDB{}
	var err error

	if rdb.db, err = db.OpenDB(dbPath); err != nil {
		return nil, err
	}
	if rdb.col = rdb.db.Use(dbCollection); rdb.col == nil {
		if err := rdb.db.Create(dbCollection); err != nil {
			return nil, err
		}
		rdb.col = rdb.db.Use(dbCollection)
	}

	return rdb, nil
}

func (rdb *RuleDB) GetAll() ([]*types.Rule, []error, error) {
	if rdb.db == nil {
		return nil, nil, fmt.Errorf("query on closed db")
	}

	var rules []*types.Rule
	var errors []error

	rdb.col.ForEachDoc(func(id int, doc []byte) (moveOn bool) {
		rule := &types.Rule{}
		var err error

		if err = json.Unmarshal(doc, &rule); err != nil {
			rule := &types.Rule{ID: id}
			rules = append(rules, rule)
			errors = append(errors, err)
		} else {
			rule.ID = id
			rules = append(rules, rule)
			errors = append(errors, nil)
		}
		return true
	})

	return rules, errors, nil
}

func (rdb *RuleDB) Get(id int) (*types.Rule, error) {
	if rdb.db == nil {
		return nil, fmt.Errorf("query on closed db")
	}

	rule := &types.Rule{}
	var doc map[string]interface{}
	var err error

	if doc, err = rdb.col.Read(id); err != nil {
		return nil, err
	}

	if err = rule.UnmarshalMap(doc); err != nil {
		return nil, err
	}

	rule.ID = id
	return rule, nil
}

func (rdb *RuleDB) Insert(rule *types.Rule) (int, error) {
	if rdb.db == nil {
		return 0, fmt.Errorf("query on closed db")
	}

	id, err := rdb.col.Insert(rule.MarshalMap())
	if err == nil {
		rule.ID = id
	}
	return id, err
}

func (rdb *RuleDB) Update(id int, rule *types.Rule) error {
	if rdb.db == nil {
		return fmt.Errorf("query on closed db")
	}

	if err := rdb.col.Update(id, rule.MarshalMap()); err != nil {
		return err
	} else {
		rule.ID = id
		return nil
	}
}

func (rdb *RuleDB) Delete(id int) error {
	if rdb.db == nil {
		return fmt.Errorf("query on closed db")
	}

	return rdb.col.Delete(id)
}

func (rdb *RuleDB) Close() error {
	err := rdb.db.Close()
	rdb.db = nil
	rdb.col = nil
	return err
}
