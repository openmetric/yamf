package main

import (
	"github.com/openmetric/yamf/scheduler"
)

func main() {
	schedulerConfig := &scheduler.Config{
		DBPath:     "/tmp/yamf",
		ListenAddr: ":8080",
	}

	scheduler.Run(schedulerConfig)
}
