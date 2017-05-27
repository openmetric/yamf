package scheduler

import (
	"fmt"
	"github.com/openmetric/yamf/internal/types"
	"math/rand"
	"sync"
	"time"
)

// Config of scheduler
type Config struct {
	ListenAddr string
	DBPath     string
}

type worker struct {
	config *Config

	publish func(*types.Task)
	rdb     *types.RuleDB
	rules   map[int]*ruleScheduler
	sync.RWMutex
}

// Run the scheduler
func Run(config *Config) {
	publish := func(t *types.Task) {
		fmt.Println("Publish Task:", t.RuleID)
	}

	rdb, _ := types.NewRuleDB(config.DBPath)

	w := &worker{
		config:  config,
		publish: publish,
		rdb:     rdb,
		rules:   make(map[int]*ruleScheduler),
	}

	w.runAPIServer()
}

type ruleScheduler struct {
	types.Rule
	stop chan struct{}
}

func (w *worker) scheduleRule(rule *types.Rule) {
	if rule.Paused {
		return
	}

	s := ruleScheduler{
		Rule: *rule,
		stop: make(chan struct{}),
	}

	go func() {
		// sleep a random time (between 0 and interval), so that checks can be distributes evenly.
		sleep := time.Duration(rand.Int63n(s.Interval.Nanoseconds())) * time.Nanosecond
		time.Sleep(sleep)

		ticker := time.NewTicker(s.Interval.Duration)
		for {
			select {
			case <-ticker.C:
				task := &types.Task{
					RuleID: s.ID,
				}
				w.publish(task)
			case <-s.stop:
				ticker.Stop()
				s.stop = nil
				return
			}
		}
	}()

	w.Lock()
	defer w.Unlock()
	w.rules[s.ID] = &s
}

func (w *worker) stopRule(id int) {
	if s, ok := w.rules[id]; ok {
		if s.stop != nil {
			close(s.stop)
		}
		w.Lock()
		defer w.Unlock()
		delete(w.rules, id)
	}
}

func (w *worker) updateRule(rule *types.Rule) {
	if oldS, ok := w.rules[rule.ID]; ok {
		if rule.Paused {
			w.stopRule(rule.ID)
		} else {
			close(oldS.stop)
			w.scheduleRule(rule)
		}
	}
}
