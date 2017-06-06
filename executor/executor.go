package executor

import (
	"fmt"
	pb "github.com/go-graphite/carbonzipper/carbonzipperpb3"
	"github.com/nsqio/go-nsq"
	api "github.com/openmetric/graphite-api-client"
	"github.com/openmetric/yamf/internal/types"
	"github.com/openmetric/yamf/logging"
	"net/http"
	"time"
)

const (
	OK       = 0
	Warning  = 1
	Critical = 2
	Unknown  = 3
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
				result, _ := e.Result.(*types.GraphiteCheckResult)
				logger.Debugf("Event, rule id: %d, status: %d, value: %v, metadata: %#v\n",
					e.RuleID, e.Status, result.MetricValue, e.Metadata)
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
	var task *types.Task
	var err error
	if task, err = types.NewTaskFromJSON(message.Body); err != nil {
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

	query = api.NewRenderQuery(api.NewQueryTarget(c.Query))
	query.SetFrom(c.From).SetUntil(c.Until)

	w.logger.Debugf("Request URL: %s", query.URL(c.GraphiteURL, "json"))
	resp, err = query.Request(c.GraphiteURL)
	if err != nil {
		w.logger.Errorf("Request to graphite server failed: %s", err)
		return
	}

	metrics := resp.MultiFetchResponse.Metrics
	w.logger.Debugf("Get %d metrics in response", len(metrics))

	metaExtractRegexp, _ := types.RegexpCompile(c.MetadataExtractPattern)
	for _, metric := range metrics {
		result := &types.GraphiteCheckResult{
			ScheduleTime:  t.Schedule,
			ExecutionTime: now,
		}
		event := &types.Event{
			Source:   "rule",
			Type:     "graphite",
			RuleID:   t.RuleID,
			Metadata: t.Metadata,
			Result:   result,
		}

		// extract meta data
		matches := metaExtractRegexp.FindStringSubmatch(metric.Name)
		names := metaExtractRegexp.SubexpNames()
		for i, match := range matches {
			if i != 0 && names[i] != "" {
				event.Metadata[names[i]] = match
			}
		}

		// compare data
		var isCritical, isWarning, isUnknown bool

		v, t, isNull := getLastNonNullValue(metric, c.AllowedNullPoints)
		result.MetricTime = time.Unix(int64(t), 0)
		result.MetricValue = v

		criticalExpr := types.NewThresholdExpr(c.CriticalExpr)
		isCritical, isUnknown = criticalExpr.Evaluate(v, isNull)
		if isUnknown {
			// emit unknown event
			event.Status = Unknown
			w.emit(event)
			continue
		} else if isCritical {
			// emit critical event
			event.Status = Critical
			w.emit(event)
			continue
		}

		warningExpr := types.NewThresholdExpr(c.WarningExpr)
		isWarning, isUnknown = warningExpr.Evaluate(v, isNull)
		if isUnknown {
			// emit unknown event
			event.Status = Unknown
			w.emit(event)
			continue
		} else if isWarning {
			// emit warning event
			event.Status = Warning
			w.emit(event)
			continue
		}

		event.Status = OK
		w.emit(event)
	}
}

func getLastNonNullValue(m *pb.FetchResponse, allowedNullPoints int) (v float64, t int32, isNull bool) {
	l := len(m.Values)
	for i := 1; i <= allowedNullPoints && i <= l; i++ {
		if m.IsAbsent[l-i] {
			continue
		}
		v = m.Values[l-i]
		t = m.StopTime - int32(i-1)*m.StepTime
		isNull = false
		return v, t, isNull
	}
	// if we didn't return in the loop body, there were too many null points
	v = 0
	t = m.StopTime
	isNull = true
	return v, t, isNull
}
