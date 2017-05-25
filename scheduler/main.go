package main

import (
	"fmt"
)

var store *RuleStore

func main() {
	ruleSpec := []byte(`
{
	"type": "graphite",
	"timeout": "2m",
	"interval": "2s",
	"metadata": { "rule-id": "test.check" },
	"graphite_task": {
		"query": "pm.*.agg.cpu.percent",
		"from": "-5min",
		"until": "now",
		"meta_extract_pattern": "^$",
		"critical_expr": "> -1.0",
		"warning_expr": "> 0.8"
	},

	"dummy": "dummy"
}
`)

	var err error
	var rule *Rule

	rule, err = NewRuleFromJSON(ruleSpec)
	if err != nil {
		fmt.Println(err)
	} else {
		fmt.Printf("%#v\n", rule)
	}

	if store, err = NewRuleStore("/tmp/store"); err != nil {
		panic(err)
	}

	//err = store.SaveRule(rule)
	//fmt.Println("rule saved, id:", rule.ID)
	//err = store.SaveRule(rule)
	//fmt.Println("rule saved, id:", rule.ID)
	//err = store.SaveRule(rule)
	//fmt.Println("rule saved, id:", rule.ID)

	rules, err := store.LoadRules()
	fmt.Println("number of rules:", len(rules))
	//for _, rule := range rules {
	//fmt.Println("rule id:", rule.ID, "query:", rule.GraphiteTask.Query)
	//rule.StartScheduling()

	//time.Sleep(10 * time.Second)

	//fmt.Println("stopping the rule")
	//rule.StopScheduling()
	//fmt.Println("rule stopped")

	//}

	//if err = store.Close(); err != nil {
	//fmt.Println(err)
	//}

	//time.Sleep(10 * time.Second)

	RunAPIServer(":8080")
}
