package scheduler

import (
	"encoding/json"
	"github.com/nsqio/go-nsq"
	"github.com/openmetric/yamf/internal/types"
	"github.com/openmetric/yamf/logging"
	"math/rand"
	"os"
	"sync"
	"time"
)

// Config of scheduler
type Config struct {
	ListenAddr      string                `yaml:"listen_addr"`
	DBPath          string                `yaml:"db_path"`
	NSQDTcpAddr     string                `yaml:"nsqd_tcp_address"`
	NSQDTopic       string                `yaml:"nsqd_topic"`
	Log             *logging.LoggerConfig `yaml:"log"`
	HTTPLogFilename string                `yaml:"http_log_filename"`
}

type worker struct {
	config *Config

	rdb      *types.RuleDB
	producer *nsq.Producer
	rules    map[int]*ruleScheduler
	sync.RWMutex

	logger *logging.Logger
}

// Run the scheduler
func Run(config *Config) {
	logger := logging.GetLogger("scheduler", config.Log)

	rdb, err := types.NewRuleDB(config.DBPath)
	if err != nil {
		logger.Fatal("error initializing rule db: ", err)
		os.Exit(1)
	}

	nsqdConfig := nsq.NewConfig()
	producer, err := nsq.NewProducer(config.NSQDTcpAddr, nsqdConfig)
	if err != nil {
		logger.Fatal("error initializing nsqd producer: ", err)
		os.Exit(1)
	}

	w := &worker{
		config:   config,
		rdb:      rdb,
		producer: producer,
		rules:    make(map[int]*ruleScheduler),
		logger:   logger,
	}

	// get all rules from db and start scheduling
	rules, err := rdb.GetAll()
	if err != nil {
		// TODO process errors
		w.logger.Error("Failed to GetAll() rules, err: ", err)
	} else {
		w.logger.Infof("Loaded %d rules from db", len(rules))
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

	w.logger.Info("Start scheduling rule:", rule.ID)

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

func (w *worker) publish(t *types.Task) {
	w.logger.Info("Publish Task:", t.RuleID, "metadata", t.Metadata)

	// TODO consider use more efficient serialization methods, e.g. protobuf
	if data, err := json.Marshal(t); err != nil {
		w.logger.Error("Failed to marshal task into json")
	} else {
		w.producer.Publish(w.config.NSQDTopic, data)
	}
}
