package scheduler

import (
	"encoding/json"
	"github.com/nsqio/go-nsq"
	"github.com/openmetric/graphite-client"
	"github.com/openmetric/yamf/internal/ruledb"
	"github.com/openmetric/yamf/internal/stats"
	"github.com/openmetric/yamf/internal/types"
	"go.uber.org/zap"
	"math/rand"
	"sync"
	"time"
)

type Config struct {
	// API Server listen address
	ListenAddress string `yaml:"listen_address"`

	// tiedot database
	DBPath       string `yaml:"db_path"`
	DBCollection string `yaml:"db_collection"`

	// nsqd and topic to publish task to
	NSQDTcpAddr string `yaml:"nsqd_tcp_address"`
	NSQTopic    string `yaml:"nsq_topic"`
}

func NewConfig() *Config {
	return &Config{
		ListenAddress: ":8080",
		DBPath:        "./var/db",
		DBCollection:  "rules",
		NSQDTcpAddr:   "127.0.0.1:4150",
		NSQTopic:      "yamf_tasks",
	}
}

// Scheduler implements main.Module
type Scheduler struct {
	config   *Config
	logger   *zap.SugaredLogger
	producer *nsq.Producer
	rdb      *ruledb.RuleDB
	stats    Stats

	apiServerStop chan struct{}

	rules map[int]*RunningRule
	sync.RWMutex
}

func NewScheduler(config *Config, logger *zap.SugaredLogger) (*Scheduler, error) {
	// TODO check if config is valid
	scheduler := &Scheduler{
		config: config,
		logger: logger,
		rules:  make(map[int]*RunningRule),
	}
	return scheduler, nil
}

func (s *Scheduler) Name() string {
	return "scheduler"
}

func (s *Scheduler) Start() error {
	// things todo
	//  * setup nsq producer
	//  * load all rules from db and start scheduling
	//  * start api server

	// setup nsq producer
	nsqdConfig := nsq.NewConfig()
	if producer, err := nsq.NewProducer(s.config.NSQDTcpAddr, nsqdConfig); err != nil {
		s.logger.Fatalw("Failed to create nsq producer.", "Error", err)
	} else {
		s.producer = producer
	}

	// open database
	if rdb, err := ruledb.NewRuleDB(s.config.DBPath, s.config.DBCollection); err != nil {
		s.logger.Fatalw("Failed to open database.", "Error", err)
	} else {
		s.rdb = rdb
	}

	// load all rules from database and run
	if rules, errors, err := s.rdb.GetAll(); err != nil {
		s.logger.Fatalw("Failed to fetch all rules from database.", "Error", err)
	} else {
		for i, rule := range rules {
			if errors[i] != nil {
				s.logger.Errorw("Error reading rule from db.", "Rule ID", rule.ID, "Error", errors[i])
			} else {
				s.schedule(rule)
			}
		}
	}

	// start api server
	s.runAPIServer()

	return nil
}

func (s *Scheduler) Stop() {
	// stop api server
	s.stopAPIServer()

	// stop all running rules
	s.logger.Info("Stopping all running rules...")

	ids := make([]int, 0, len(s.rules))
	for id, _ := range s.rules {
		ids = append(ids, id)
	}

	for _, id := range ids {
		s.stop(id)
	}
}

func (s *Scheduler) GatherStats() []*graphite.Metric {
	return stats.ToGraphiteMetric(s.stats, "")
}

func (s *Scheduler) schedule(r *types.Rule) {
	s.stop(r.ID)
	s.start(r)
}

func (s *Scheduler) stop(id int) {
	s.Lock()
	defer s.Unlock()
	if r, ok := s.rules[id]; ok {
		if r.stop != nil {
			s.logger.Infow("Stop scheduling rule", "Rule ID", r.ID)
			s.stats.ActiveRules.Dec()
			close(r.stop)
		}
		delete(s.rules, id)
	}
}

func (s *Scheduler) start(rule *types.Rule) {
	if rule.Paused {
		s.logger.Infow("Rule is paused, not scheduling", "Rule ID", rule.ID)
		return
	}

	s.stats.ActiveRules.Inc()
	s.logger.Infow("Start scheduling rule", "Rule ID", rule.ID)

	r := &RunningRule{
		Rule: rule,
	}
	r.stop = make(chan struct{})

	s.Lock()
	defer s.Unlock()
	s.rules[r.ID] = r
	go func() {
		// sleep a random time (between 0 and interval), so that checks can be distributed evenly.
		sleep := time.Duration(rand.Int63n(r.Interval.Nanoseconds())) * time.Nanosecond
		time.Sleep(sleep)

		ticker := time.NewTicker(r.Interval.Duration)
		for {
			select {
			case <-ticker.C:
				s.emitTask(r.Rule)
			case <-r.stop:
				ticker.Stop()
				r.stop = nil
				return
			}
		}
	}()
}

func (s *Scheduler) emitTask(rule *types.Rule) {
	s.stats.TaskScheduled.Inc()
	t := types.NewTaskFromRule(rule)

	s.logger.Debugw("Emitting task.", "Rule ID", t.RuleID)

	if data, err := json.Marshal(t); err != nil {
		s.logger.Errorw("Failed to marshal task into json.", "Error", err)
	} else {
		s.producer.Publish(s.config.NSQTopic, data)
	}
}

type RunningRule struct {
	*types.Rule

	stop chan struct{}
}
