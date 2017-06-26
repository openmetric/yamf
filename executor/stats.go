package executor

import (
	"github.com/openmetric/yamf/internal/stats"
)

type Stats struct {
	TaskReceived stats.Counter `stats:"TaskReceived"`
	TaskExecuted stats.Counter `stats:"TaskExecuted"`
	TaskExpired  stats.Counter `stats:"TaskExpired"`
	EventEmitted stats.Counter `stats:"EventEmitted"`

	EventOK       stats.Counter `stats:"EventOK"`
	EventWarning  stats.Counter `stats:"EventWarning"`
	EventCritical stats.Counter `stats:"EventCritical"`
	EventUnknown  stats.Counter `stats:"EventUnknown"`

	GraphiteExecutor struct {
		TaskExecuted     stats.Counter `stats:"TaskExecuted"`
		EventEmitted     stats.Counter `stats:"EventEmitted"`
		APIRequestTotal  stats.Counter `stats:"APIRequestTotal"`
		APIRequestFailed stats.Counter `stats:"APIRequestFailed"`
		MetricsReceived  stats.Counter `stats:"MetricsReceived"`

		EventOK       stats.Counter `stats:"EventOK"`
		EventWarning  stats.Counter `stats:"EventWarning"`
		EventCritical stats.Counter `stats:"EventCritical"`
		EventUnknown  stats.Counter `stats:"EventUnknown"`
	} `stats:"GraphiteExecutor"`
}
