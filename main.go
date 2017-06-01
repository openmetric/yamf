package main

import (
	"flag"
	"fmt"
	"github.com/openmetric/yamf/logging"
	"github.com/openmetric/yamf/scheduler"
	"os"
)

func main() {
	configFile := flag.String("config", "", "Path to the `config file`.")
	mode := flag.String("mode", "", "Modes: scheduler, executor")
	flag.Parse()

	config := loadConfig(*configFile)
	logger := logging.GetLogger("main", config.Log)

	switch *mode {
	case "scheduler":
		logger.Info("Running as scheduler")
		logger.Info("Starting scheduler")
		scheduler.Run(config.Scheduler)
	case "executor":
		fmt.Println("Executor not implemented")
		os.Exit(1)
	default:
		fmt.Println("You must specify a valid mode with `-mode` option")
		os.Exit(1)
	}
}
