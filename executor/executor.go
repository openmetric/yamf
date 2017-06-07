package executor

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/nsqio/go-nsq"
	api "github.com/openmetric/graphite-api-client"
	"github.com/openmetric/yamf/internal/types"
	"github.com/openmetric/yamf/logging"
	"net/http"
	"time"
)

// Config of executor
type Config struct {
	NumWorkers         int                   `yaml:"num_workers"`
	NSQLookupdHTTPAddr string                `yaml:"nsqlookupd_http_address"`
	NSQTopic           string                `yaml:"nsq_topic"`
	NSQChannel         string                `yaml:"nsq_channel"`
	Log                *logging.LoggerConfig `yaml:"log"`
}

type worker struct {
	config *Config
	id     int // worker id, used in log
	logger *logging.Logger

	consumer   *nsq.Consumer
	httpclient *http.Client

	emit func(*types.Event)

	stop chan struct{}
}

var workers []*worker
var logger *logging.Logger

// Run the executor
func Run(config *Config) {
	logger = logging.GetLogger("executor", config.Log)
	workers = make([]*worker, config.NumWorkers)

	for i := 0; i < config.NumWorkers; i++ {
		name := fmt.Sprintf("executor-%d", i)
		workers[i] = &worker{
			config: config,
			id:     i,
			logger: logging.GetLogger(name, config.Log),

			emit: func(e *types.Event) {
				//result, _ := e.Result.(*types.GraphiteResult)
				b, _ := json.Marshal(e)
				logger.Debugf(string(b))
			},

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
	w.consumer.AddHandler(nsq.HandlerFunc(w.executeTask))
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

func (w *worker) executeTask(message *nsq.Message) error {
	var err error
	task := &types.Task{}
	if err = json.Unmarshal(message.Body, task); err != nil {
		w.logger.Errorf("failed to decode task from message, %s", err)
	} else {
		w.logger.Debugf("get task, rule id: %d", task.RuleID)

		switch c := task.Check.(type) {
		case *types.GraphiteCheck:
			w.executeGraphiteCheck(c, task)
		}
	}
	return nil
}

func (w *worker) executeGraphiteCheck(c *types.GraphiteCheck, t *types.Task) {
	var query *api.RenderQuery
	var resp *api.RenderResponse
	var err error

	now := time.Now()

	query = api.NewRenderQuery(c.GraphiteURL, c.From, c.Until, api.NewRenderTarget(c.Query))

	w.logger.Debugf("Request URL: %s", query.URL())
	ctx, cancel := context.WithTimeout(context.TODO(), t.Timeout.Duration)
	defer cancel()
	resp, err = query.Request(ctx)
	if err != nil {
		w.logger.Errorf("Request to graphite server failed: %s", err)
		return
	}

	metrics := resp.MultiFetchResponse.Metrics
	w.logger.Debugf("Get %d metrics in response", len(metrics))

	metaExtractRegexp, _ := types.RegexpCompile(c.MetadataExtractPattern)
	for _, metric := range metrics {
		result := &types.GraphiteResult{
			MetricName: metric.GetName(),
			Metadata:   make(map[string]interface{}),
		}
		event := types.NewEvent("rule")
		event.Type = "graphite"
		event.RuleID = t.RuleID
		event.Metadata = t.Metadata
		event.Timestamp = now
		event.IdentifierTemplate = types.NewIdentifierTemplate(t.EventIdentifierPattern)

		// extract meta data
		matches := metaExtractRegexp.FindStringSubmatch(metric.Name)
		names := metaExtractRegexp.SubexpNames()
		for i, match := range matches {
			if i != 0 && names[i] != "" {
				result.Metadata[names[i]] = match
			}
		}

		// compare data
		var isCritical, isWarning, isUnknown bool

		v, t, absent := api.GetLastNonNullValue(metric, c.MaxNullPoints)
		result.MetricTimestamp = time.Unix(int64(t), 0)
		result.MetricValue = v

		isCritical, isUnknown = c.CriticalExpression.Evaluate(v, absent)
		if isUnknown {
			// emit unknown event
			result.Status = types.Unknown
			event.SetResult(result)
			w.emit(event)
			continue
		} else if isCritical {
			// emit critical event
			result.Status = types.Critical
			event.SetResult(result)
			w.emit(event)
			continue
		}

		isWarning, isUnknown = c.WarningExpression.Evaluate(v, absent)
		if isUnknown {
			// emit unknown event
			result.Status = types.Unknown
			event.SetResult(result)
			w.emit(event)
			continue
		} else if isWarning {
			// emit warning event
			result.Status = types.Warning
			event.SetResult(result)
			w.emit(event)
			continue
		}

		result.Status = types.OK
		event.SetResult(result)
		w.emit(event)
	}
}
