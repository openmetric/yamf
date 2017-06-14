package executor

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/nsqio/go-nsq"
	api "github.com/openmetric/graphite-api-client"
	"github.com/openmetric/yamf/internal/types"
	"github.com/openmetric/yamf/logging"
	"time"
)

type Config struct {
	NumWorkers         int                   `yaml:"num_workers"`
	NSQLookupdHTTPAddr string                `yaml:"nsqlookupd_http_address"`
	NSQTopic           string                `yaml:"nsq_topic"`
	NSQChannel         string                `yaml:"nsq_channel"`
	Log                *logging.LoggerConfig `yaml:"log"`

	EmitType        string `yaml:"emit_type"`
	EmitFilename    string `yaml:"emit_filename"`
	EmitNSQDTCPAddr string `yaml:"emit_nsqd_tcp_address"`
	EmitNSQTopic    string `yaml:"emit_nsq_topic"`
	FilterMode      int    `yaml:"filter_mode"`
}

type executorWorker struct {
	config   *Config
	logger   *logging.Logger
	consumer *nsq.Consumer
	emitter  Emitter
	filter   *eventFilter
	stop     chan struct{}
}

var workers []*executorWorker
var logger *logging.Logger

func Run(config *Config) {
	logger = logging.GetLogger("executor", config.Log)
	workers = make([]*executorWorker, config.NumWorkers)

	var emitter Emitter
	switch config.EmitType {
	case "file":
		emitter = NewFileEmitter(config.EmitFilename)
	case "nsq":
		emitter = NewNSQEmitter(config.EmitNSQDTCPAddr, config.EmitNSQTopic)
	}

	filter := NewEventFilter(config.FilterMode)

	for i := 0; i < config.NumWorkers; i++ {
		name := fmt.Sprintf("executor-%d", i)
		workers[i] = &executorWorker{
			config:  config,
			logger:  logging.GetLogger(name, config.Log),
			emitter: emitter,
			filter:  filter,
			stop:    make(chan struct{}),
		}
		workers[i].Start()
	}
}

func Stop() {
	for _, w := range workers {
		w.Stop()
	}
}

func (w *executorWorker) Start() {
	var err error
	nsqConfig := nsq.NewConfig()
	if w.consumer, err = nsq.NewConsumer(w.config.NSQTopic, w.config.NSQChannel, nsqConfig); err != nil {
		w.logger.Fatal("Failed with nsq.NewConsumer()")
	}
	w.consumer.AddHandler(nsq.HandlerFunc(w.doTask))
	if err = w.consumer.ConnectToNSQLookupd(w.config.NSQLookupdHTTPAddr); err != nil {
		w.logger.Fatal("Failed to connect to nsqlookupd")
	}
}

func (w *executorWorker) Stop() {
	if w.stop != nil {
		close(w.stop)
		w.stop = nil
	}
	if w.consumer != nil {
		w.consumer.Stop()
		<-w.consumer.StopChan
	}
}

func (w *executorWorker) doTask(message *nsq.Message) error {
	var err error
	task := &types.Task{}
	if err = json.Unmarshal(message.Body, task); err != nil {
		w.logger.Errorf("failed to decode task from message: %s", err)
	} else {
		w.logger.Debugf("get task, rule id: %d", task.RuleID)

		switch task.Type {
		case "graphite":
			w.doGraphiteTask(task)
		}
	}
	return nil
}

func (w *executorWorker) doGraphiteTask(task *types.Task) {
	var query *api.RenderQuery
	var resp *api.RenderResponse
	var err error

	now := time.Now()
	if now.After(task.Expiration.Time) {
		w.logger.Warning("Expired task, RuleID: %d, Schedule Time: %v, Expiration: %s, Now: %v",
			task.RuleID, task.Schedule, task.Expiration, now)
		return
	}

	check := task.Check.(*types.GraphiteCheck)
	query = api.NewRenderQuery(check.GraphiteURL, check.From, check.Until, api.NewRenderTarget(check.Query))

	w.logger.Debugf("Request URL: %s", query.URL())
	ctx, cancel := context.WithDeadline(context.TODO(), task.Deadline.Time)
	defer cancel()
	resp, err = query.Request(ctx)
	if err != nil {
		w.logger.Errorf("Request to graphite server failed: %s", err)
		return
	}

	metrics := resp.MultiFetchResponse.Metrics
	w.logger.Debugf("Got %d metrics in response", len(metrics))

	metaExtractRegexp, _ := types.RegexpCompile(check.MetadataExtractPattern)
	for _, metric := range metrics {
		result := types.NewGraphiteResult()
		result.CheckTimestamp = types.FromTime(now)

		matches := metaExtractRegexp.FindStringSubmatch(metric.Name)
		names := metaExtractRegexp.SubexpNames()
		for i, match := range matches {
			if i != 0 && names[i] != "" {
				result.Metadata[names[i]] = match
			}
		}

		var isCritical, isWarning, isUnknown bool

		v, t, absent := api.GetLastNonNullValue(metric, check.MaxNullPoints)
		result.MetricTimestamp = types.FromTime(time.Unix(int64(t), 0))
		result.MetricValue = v
		result.MetricValueAbsent = absent
		result.MetricName = metric.Name

		isCritical, isUnknown = check.CriticalExpression.Evaluate(v, absent)
		if isCritical {
			result.Status = types.Critical
			goto EMIT
		}

		isWarning, isUnknown = check.WarningExpression.Evaluate(v, absent)
		if isWarning {
			result.Status = types.Warning
			goto EMIT
		}

		if isUnknown {
			result.Status = types.Unknown
		} else {
			result.Status = types.OK
		}

	EMIT:
		event := &types.Event{
			Source:      "rule",
			Type:        "graphite",
			Timestamp:   types.FromTime(time.Now()),
			Status:      result.Status,
			Description: "",
			Metadata:    task.Metadata.Copy(),
			RuleID:      task.RuleID,
			Result:      result,
		}
		event.Metadata.Merge(result.Metadata)
		event.Identifier, _ = task.EventIdentifierPattern.Parse(event.Metadata)
		w.emit(event)
	}
}

func (w *executorWorker) emit(e *types.Event) {
	if w.emitter != nil {
		if w.filter.ShouldEmit(e) {
			w.emitter.Emit(e)
		}
	}
}
