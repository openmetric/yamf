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
		fmt.Println(t.Schedule, "Publish Task:", t.RuleID, "metadata:", t.Metadata)
	}

	rdb, _ := types.NewRuleDB(config.DBPath)

	w := &worker{
		config:  config,
		publish: publish,
		rdb:     rdb,
		rules:   make(map[int]*ruleScheduler),
	}

	// get all rules from db and start scheduling
	rules, err := rdb.GetAll()
	if err != nil {
		// TODO process errors
	} else {
		fmt.Printf("Loaded %d rules from db\n", len(rules))
		for _, rule := range rules {
			w.startSchedule(rule)
		}
	}

	w.runAPIServer()
}

type ruleScheduler struct {
	types.Rule
	stop chan struct{}
}

// start a go routine to schedule the given rule
func (w *worker) startSchedule(rule *types.Rule) {
	if rule.Paused {
		return
	}

	s := ruleScheduler{
		Rule: *rule,
		stop: make(chan struct{}),
	}

	fmt.Println("Start scheduling rule:", rule.ID)

	go func() {
		// sleep a random time (between 0 and interval), so that checks can be distributes evenly.
		sleep := time.Duration(rand.Int63n(s.Interval.Nanoseconds())) * time.Nanosecond
		time.Sleep(sleep)

		ticker := time.NewTicker(s.Interval.Duration)
		for {
			select {
			case <-ticker.C:
				w.publish(types.NewTaskFromRule(rule))
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

func (w *worker) stopSchedule(id int) {
	if s, ok := w.rules[id]; ok {
		if s.stop != nil {
			close(s.stop)
		}
		w.Lock()
		defer w.Unlock()
		delete(w.rules, id)
	}
}

func (w *worker) updateSchedule(rule *types.Rule) {
	w.stopSchedule(rule.ID)
	w.startSchedule(rule)
}
