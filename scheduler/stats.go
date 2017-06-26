package scheduler

import (
	"github.com/openmetric/yamf/internal/stats"
)

type Stats struct {
	ActiveRules   stats.Gauge   `stats:"ActiveRules"`
	TaskScheduled stats.Counter `stats:"TaskScheduled"`
}
