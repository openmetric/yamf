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
			w.executeGraphiteCheck(task, c)
		}
	}
	return nil
}

func (w *worker) executeGraphiteCheck(t *types.Task, c *types.GraphiteCheck) {
	var query *api.RenderQuery
	var resp *api.RenderResponse
	var err error

	now := time.Now()
	if now.After(t.Expiration.Time) {
		w.logger.Warning("Expired task, RuleID: %d, Schedule Time: %v, Timeout: %s, Now: %v",
			t.RuleID, t.Schedule, t.Timeout, now)
		return
	}

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
		result := types.NewGraphiteResult()
		result.CheckTimestamp = types.FromTime(now)

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

		v, ts, absent := api.GetLastNonNullValue(metric, c.MaxNullPoints)
		result.MetricTimestamp = types.FromTime(time.Unix(int64(ts), 0))
		result.MetricValue = v
		result.MetricValueAbsent = absent

		isCritical, isUnknown = c.CriticalExpression.Evaluate(v, absent)
		if isUnknown {
			result.Status = types.Unknown
			goto EMIT
		} else if isCritical {
			result.Status = types.Critical
			goto EMIT
		}

		isWarning, isUnknown = c.WarningExpression.Evaluate(v, absent)
		if isUnknown {
			// emit unknown event
			result.Status = types.Unknown
			goto EMIT
		} else if isWarning {
			// emit warning event
			result.Status = types.Warning
			goto EMIT
		}

	EMIT:
		event := &types.Event{
			Source:      "rule",
			Type:        "graphite",
			Timestamp:   types.FromTime(time.Now()),
			Status:      result.Status,
			Description: "",
			Metadata:    t.Metadata.Copy(),

			RuleID: t.RuleID,
			Result: result,
		}
		event.Metadata.Merge(result.Metadata)
		event.Identifier, _ = t.EventIdentifierPattern.Parse(event.Metadata)
		w.emit(event)
	}
}

func (w *worker) emit(e *types.Event) {
	r := e.Result.(*types.GraphiteResult)
	w.logger.Debugf("[%s][%v] %d, Value: %f (isAbsent: %t)",
		e.Identifier, e.Timestamp, e.Status, r.MetricValue, r.MetricValueAbsent,
	)
}
