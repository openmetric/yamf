package executor

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/nsqio/go-nsq"
	api "github.com/openmetric/graphite-api-client"
	"github.com/openmetric/graphite-client"
	"github.com/openmetric/yamf/internal/stats"
	"github.com/openmetric/yamf/internal/types"
	"go.uber.org/zap"
	"sync"
	"time"
)

type Config struct {
	// how many workers to run
	NumWorkers int `yaml:num_workers`

	// nsq comsumer config
	NSQLookupdHTTPAddr string `yaml:"nsqlookupd_http_address"`
	NSQTopic           string `yaml:"nsq_topic"`
	NSQChannel         string `yaml:"nsq_channel"`

	Emit *EmitConfig `yaml:"emit"`
}

func NewConfig() *Config {
	return &Config{
		NumWorkers:         1,
		NSQLookupdHTTPAddr: "127.0.0.1:4161",
		NSQTopic:           "yamf_tasks",
		NSQChannel:         "yamf_task_executor",
		Emit: &EmitConfig{
			FilterMode: 0,
			Type:       "file",
			Filename:   "/dev/stdout",
		},
	}
}

type EmitConfig struct {
	Type       string `yaml:"type"`
	FilterMode int    `yaml:"filter_mode"`

	// file emitter
	Filename string `yaml:"filename"`

	// nsq emitter
	NSQDTCPAddr string `yaml:"nsqd_tcp_address"`
	NSQTopic    string `yaml:"nsq_topic"`

	// rabbitmq emitter
	RabbitMQUri   string `yaml:"rabbitmq_uri"`
	RabbitMQQueue string `yaml:"rabbitmq_queue"`
}

type Executor struct {
	config  *Config
	logger  *zap.SugaredLogger
	emitter Emitter
	filter  *eventFilter
	stats   Stats

	workerStops []chan struct{}
	workerWG    *sync.WaitGroup
}

func NewExecutor(config *Config, logger *zap.SugaredLogger) (*Executor, error) {
	executor := &Executor{
		config: config,
		logger: logger,

		workerWG: new(sync.WaitGroup),
	}
	return executor, nil
}

func (e *Executor) Name() string {
	return "executor"
}

func (e *Executor) Start() error {
	var err error
	if e.emitter, err = NewEmitter(e.config.Emit); err != nil {
		return fmt.Errorf("failed to initialize emitter: %s", err)
	}
	if e.filter, err = NewEventFilter(e.config.Emit.FilterMode); err != nil {
		return fmt.Errorf("failed to create event filter: %s", err)
	}

	for i := 0; i < e.config.NumWorkers; i++ {
		stop := make(chan struct{})
		e.workerStops = append(e.workerStops, stop)
		e.workerWG.Add(1)
		go e.runWorker(stop)
	}

	return nil
}

func (e *Executor) Stop() {
	// stop all workers
	for _, stop := range e.workerStops {
		close(stop)
	}
	e.workerWG.Wait()
	e.logger.Info("executor stopped.")
}

func (e *Executor) GatherStats() []*graphite.Metric {
	return stats.ToGraphiteMetric(e.stats, "")
}

func (e *Executor) runWorker(stop chan struct{}) {
	defer e.workerWG.Done()

	// setup consumer
	var err error
	var consumer *nsq.Consumer

	nsqConfig := nsq.NewConfig()
	if consumer, err = nsq.NewConsumer(e.config.NSQTopic, e.config.NSQChannel, nsqConfig); err != nil {
		e.logger.Fatalw("Failed to initialize nsq consumer.", "Error", err)
	}
	consumer.AddHandler(nsq.HandlerFunc(e.doTask))
	if err = consumer.ConnectToNSQLookupd(e.config.NSQLookupdHTTPAddr); err != nil {
		e.logger.Fatalw("Failed to connect to nsqlookupd.", "Error", err)
	}

	// wait for stop
	<-stop
	consumer.Stop()
	<-consumer.StopChan
}

func (e *Executor) doTask(message *nsq.Message) error {
	var err error
	task := &types.Task{}

	e.stats.TaskReceived.Inc()

	if err = json.Unmarshal(message.Body, task); err != nil {
		e.logger.Errorw("Failed to decode task from message.", "Error", err)
	} else {
		now := time.Now()
		if now.After(task.Expiration.Time) {
			e.stats.TaskExpired.Inc()
			e.logger.Warnw(
				"Expired task.",
				"Rule ID", task.RuleID,
				"Schedule", task.Schedule,
				"Expiration:", task.Expiration,
				"Now", now,
			)
			return nil
		}

		e.logger.Debugw("Start doing task", "Rule ID", task.RuleID, "Task Type", task.Type)
		switch task.Type {
		case "graphite":
			e.doGraphiteTask(task)
		}
		e.stats.TaskExecuted.Inc()
	}
	return nil
}

func (e *Executor) doGraphiteTask(task *types.Task) {
	var query *api.RenderQuery
	var resp *api.RenderResponse
	var err error

	defer e.stats.GraphiteExecutor.TaskExecuted.Inc()

	begin := time.Now()
	check := task.Check.(*types.GraphiteCheck)
	query = api.NewRenderQuery(check.GraphiteURL, check.From, check.Until, api.NewRenderTarget(check.Query))

	e.logger.Debugw("Querying graphite.", "URL", query.URL())
	ctx, cancel := context.WithDeadline(context.TODO(), task.Deadline.Time)
	e.stats.GraphiteExecutor.APIRequestTotal.Inc()
	defer cancel()

	if resp, err = query.Request(ctx); err != nil {
		e.stats.GraphiteExecutor.APIRequestFailed.Inc()
		e.logger.Errorw("Request to graphite server failed.", "Error", err)
		return
	}

	metrics := resp.MultiFetchResponse.Metrics
	e.stats.GraphiteExecutor.MetricsReceived.Add(uint64(len(metrics)))
	e.logger.Debugw("Got graphite render response.", "N Metrics", len(metrics))

	metaExtractRegexp, _ := types.RegexpCompile(check.MetadataExtractPattern)
	for _, metric := range metrics {
		result := types.NewGraphiteResult()
		result.CheckTimestamp = types.FromTime(begin)

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

		switch result.Status {
		case types.OK:
			e.stats.GraphiteExecutor.EventOK.Inc()
		case types.Warning:
			e.stats.GraphiteExecutor.EventWarning.Inc()
		case types.Critical:
			e.stats.GraphiteExecutor.EventCritical.Inc()
		case types.Unknown:
			e.stats.GraphiteExecutor.EventUnknown.Inc()
		}

		e.stats.GraphiteExecutor.EventEmitted.Inc()
		e.emitEvent(event)
	}
}

func (e *Executor) emitEvent(event *types.Event) {
	switch event.Status {
	case types.OK:
		e.stats.EventOK.Inc()
	case types.Warning:
		e.stats.EventWarning.Inc()
	case types.Critical:
		e.stats.EventCritical.Inc()
	case types.Unknown:
		e.stats.EventUnknown.Inc()
	}
	e.stats.EventEmitted.Inc()
	if e.emitter != nil {
		if e.filter.ShouldEmit(event) {
			e.emitter.Emit(event)
		}
	}
}
