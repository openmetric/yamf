package main

import ()

var store *RuleStore

func main() {
	var err error
	if store, err = NewRuleStore("/tmp/store"); err != nil {
		panic(err)
	}

	RunAPIServer(":8080")
}
