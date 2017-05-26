package main

import (
	"fmt"
	"sync"
)

type SchedulerConfig struct {
	ListenAddr string `yaml:"listen_addr"`
	DBPath     string `yaml:"db_path"`
}

type ScheduleManager struct {
	rules   map[int]*Rule
	publish func(*Task)
	sync.RWMutex
}

func (m *ScheduleManager) AddRule(rule *Rule) {
	if _, ok := m.rules[rule.ID]; ok {
		// TODO log error
		return
	}

	if rule.Paused == true {
		return
	}

	m.Lock()
	defer m.Unlock()
	rule.Start(m.publish)
	m.rules[rule.ID] = rule
}

func (m *ScheduleManager) RemoveRule(id int) {
	if rule, ok := m.rules[id]; ok {
		m.Lock()
		defer m.Unlock()
		rule.Stop()
		delete(m.rules, id)
	} else {
		// TODO log error
	}
}

func (m *ScheduleManager) UpdateRule(rule *Rule) {
	if oldRule, ok := m.rules[rule.ID]; ok {
		m.Lock()
		defer m.Unlock()
		oldRule.Stop()
		if rule.Paused == true {
			delete(m.rules, rule.ID)
		} else {
			rule.Start(m.publish)
			m.rules[rule.ID] = rule
		}

	} else {
		// TODO log error
	}
}

var schedulerManager *ScheduleManager
var ruleDB *RuleDB

func RunScheduler() {
	schedulerManager = &ScheduleManager{
		rules: make(map[int]*Rule),
		publish: func(t *Task) {
			fmt.Println("Publish Task, rule id:", t.RuleID)
		},
	}

	ruleDB, _ = NewRuleDB("/tmp/yamf")

	// load and run all rules

	// start http api
	fmt.Println("starting api server")
	RunAPIServer(":8080")
	fmt.Println("api server stopped")
}
