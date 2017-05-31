package main

import (
	"github.com/openmetric/yamf/logging"
	"github.com/openmetric/yamf/scheduler"
)

func main() {
	schedulerConfig := &scheduler.Config{
		DBPath:       "./var/db",
		ListenAddr:   ":8080",
		APIUrlPrefix: "v1",
		Log: &logging.LoggerConfig{
			Filename: "./var/log/scheduler.log",
			Level:    "info",
		},
	}

	scheduler.Run(schedulerConfig)
}
