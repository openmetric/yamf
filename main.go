package main

import (
	"flag"
	"fmt"
	"github.com/openmetric/yamf/executor"
	"github.com/openmetric/yamf/logging"
	"github.com/openmetric/yamf/scheduler"
	"os"
	"os/signal"
	"syscall"
)

type Module interface {
	Start() error
	Stop()
	GatherStats()
}

func main() {
	configFile := flag.String("config", "", "Path to the `config file`.")
	mode := flag.String("mode", "", "Modes: scheduler, executor")
	flag.Parse()

	config := loadConfig(*configFile)
	logger := logging.GetLogger("main", config.Log)

	var module Module
	var err error

	switch *mode {
	case "scheduler":
		logger.Info("Running as scheduler")
		logger.Info("Starting scheduler")
		if module, err = scheduler.NewScheduler(config.Scheduler, logger); err != nil {
			logger.Panicf("Failed to initialize scheduler: %s", err)
		}
		if err = module.Start(); err != nil {
			logger.Panicf("Failed to start scheduler: %s", err)
		}
	case "executor":
		logger.Info("Running as executor")
		logger.Info("Starting executor")
		if module, err = executor.NewExecutor(config.Executor, logger); err != nil {
			logger.Panicf("Failed to initialize executor: %s", err)
		}
		if err = module.Start(); err != nil {
			logger.Panicf("Failed to start executor: %s", err)
		}
	default:
		fmt.Println("You must specify a valid mode with `-mode` option")
		os.Exit(1)
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)

	for {
		select {
		case <-c:
			logger.Info("Got stop signal, stopping...")
			switch *mode {
			case "scheduler", "executor":
				module.Stop()
			}
			return
		}
	}
}
