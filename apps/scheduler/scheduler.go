package main

import (
	"flag"
	"fmt"
	"github.com/openmetric/graphite-client"
	"github.com/openmetric/yamf/internal/logging"
	"github.com/openmetric/yamf/internal/stats"
	"github.com/openmetric/yamf/internal/utils"
	"github.com/openmetric/yamf/scheduler"
	"go.uber.org/zap"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	configFile := flag.String("config", "", "Path to the `config file`.")
	flag.Parse()

	var err error
	var module *scheduler.Scheduler
	var logger *zap.SugaredLogger

	config := struct {
		Scheduler *scheduler.Config `yaml:"scheduler"`
		Log       *logging.Config   `yaml:"log"`
		Stats     *stats.Config     `yaml:"stats"`
	}{
		Scheduler: scheduler.NewConfig(),
		Log:       logging.NewConfig(),
		Stats:     stats.NewConfig(),
	}
	if err := utils.UnmarshalYAMLFile(*configFile, config); err != nil {
		panic(fmt.Sprintf("Error reading config file: %s", err))
	}

	if logger, err = logging.NewLogger(config.Log); err != nil {
		panic(fmt.Sprintf("Error initializing logger: %s", err))
	}

	if module, err = scheduler.NewScheduler(config.Scheduler, logger); err != nil {
		logger.Panicw("Error initializing scheduler.", "Error", err)
	}

	var statsClient *graphite.Client
	if config.Stats.Enabled {
		statsClient, err = graphite.NewClient(
			config.Stats.URL,
			config.Stats.Prefix,
			time.Second,
		)
		if err != nil {
			logger.Panicw("Error initializing graphite client for internal stats", "Error", err)
		}
	}

	if err = module.Start(); err != nil {
		logger.Panicw("Error starting scheduler.", "Error", err)
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)

	var statsTicker <-chan time.Time
	if config.Stats.Enabled {
		statsTicker = time.Tick(config.Stats.Interval)
	} else {
		statsTicker = make(chan time.Time)
	}

	for {
		select {
		case <-c:
			logger.Info("Got stop signal, stopping scheduler...")
			module.Stop()
			return
		case <-statsTicker:
			metrics := module.GatherStats()
			statsClient.SendMetrics(metrics)
		}
	}
}
