package types

type Result interface{}

type GraphiteResult struct {
	Status            int      `json:"status"`
	CheckTimestamp    Time     `json:"check_timestamp"` // when the check was performed
	MetricName        string   `json:"metric_name"`
	MetricTimestamp   Time     `json:"metric_timestamp"`
	MetricValue       float64  `json:"metric_value"`
	MetricValueAbsent bool     `json:"metric_value_absent"`
	Metadata          Metadata `json:"metadata"` // data extracted
}

func NewGraphiteResult() *GraphiteResult {
	return &GraphiteResult{
		Metadata: make(Metadata),
	}
}
