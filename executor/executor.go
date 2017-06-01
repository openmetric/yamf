package executor

import (
	"github.com/nsqio/go-nsq"
	"github.com/openmetric/yamf/internal/types"
	"github.com/openmetric/yamf/logging"
	"net/http"
)

// Config of executor
type Config struct {
	NumWorkers            int                   `yaml:"num_workers"`
	NSQLookupdHTTPAddr    string                `yaml:"nsqlookupd_http_address"`
	NSQTopic              string                `yaml:"nsq_topic"`
	NSQChannel            string                `yaml:"nsq_channel"`
	DefaultGraphiteAPIURL string                `yaml:"default_graphite_api_url"`
	Log                   *logging.LoggerConfig `yaml:"log"`
}

type worker struct {
	config *Config
	id     int // worker id, used in log
	logger *logging.Logger

	consumer   *nsq.Consumer
	httpclient *http.Client

	stop chan struct{}
}

var workers []*worker
var logger *logging.Logger

// Run the executor
func Run(config *Config) {
	logger = logging.GetLogger("executor", config.Log)
	workers = make([]*worker, config.NumWorkers)

	for i := 0; i < config.NumWorkers; i++ {
		workers[i] = &worker{
			config: config,
			id:     i,
			logger: logger,

			stop: make(chan struct{}),
		}
		workers[i].Start()
	}
}

// Stop all executor workers
func Stop() {
	for _, w := range workers {
		w.Stop()
	}
}

func (w *worker) Start() {
	var err error
	nsqConfig := nsq.NewConfig()
	if w.consumer, err = nsq.NewConsumer(w.config.NSQTopic, w.config.NSQChannel, nsqConfig); err != nil {
		w.logger.Fatal("Failed with nsq.NewConsumer()")
	}
	w.consumer.AddHandler(nsq.HandlerFunc(w.ExecuteTask))
	if err = w.consumer.ConnectToNSQLookupd(w.config.NSQLookupdHTTPAddr); err != nil {
		w.logger.Fatal("Failed to connect to nsqlookupd")
	}
}

func (w *worker) Stop() {
	if w.stop != nil {
		close(w.stop)
		w.stop = nil
	}

	if w.consumer != nil {
		w.consumer.Stop()
		<-w.consumer.StopChan
	}
}

func (w *worker) ExecuteTask(message *nsq.Message) error {
	var task *types.Task
	var err error
	if task, err = types.NewTaskFromJSON(message.Body); err != nil {
		w.logger.Errorf("[executor-%d] failed to decode task from message, %s", w.id, err)
	} else {
		w.logger.Debugf("[executor-%d] get task, rule id: %d", w.id, task.RuleID)
	}
	return nil
}