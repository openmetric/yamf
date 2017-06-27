package main

import (
	"flag"
	"fmt"
	"github.com/openmetric/graphite-client"
	"github.com/openmetric/yamf/executor"
	"github.com/openmetric/yamf/internal/logging"
	"github.com/openmetric/yamf/internal/stats"
	"github.com/openmetric/yamf/internal/utils"
	"github.com/openmetric/yamf/scheduler"
	"go.uber.org/zap"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

var BuildVersion = "(development build)"

type Module interface {
	Name() string
	Start() error
	Stop()
	GatherStats() []*graphite.Metric
}

func main() {
	configFile := flag.String("config", "", "Path to the `config file`.")
	printVersion := flag.Bool("version", false, "Print version and exit.")
	flag.Parse()

	if *printVersion {
		fmt.Printf("yamf version: %s\n", BuildVersion)
		os.Exit(0)
	}

	var err error
	var module Module
	var logger *zap.SugaredLogger

	var mode string
	switch {
	case strings.HasSuffix(os.Args[0], "executor"):
		mode = "executor"
	case strings.HasSuffix(os.Args[0], "scheduler"):
		mode = "scheduler"
	}

	config := struct {
		Mode      string            `yaml:"mode"`
		Executor  *executor.Config  `yaml:"executor"`
		Scheduler *scheduler.Config `yaml:"scheduler"`
		Log       *logging.Config   `yaml:"log"`
		Stats     *stats.Config     `yaml:"stats"`
	}{
		Mode:      mode,
		Executor:  executor.NewConfig(),
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

	logger.Infof("yamf version: %s", BuildVersion)

	switch config.Mode {
	case "executor":
		if module, err = executor.NewExecutor(config.Executor, logger); err != nil {
			logger.Panicw("Error initializing executor.", "Error", err)
		}
	case "scheduler":
		if module, err = scheduler.NewScheduler(config.Scheduler, logger); err != nil {
			logger.Panicw("Error initializing scheduler.", "Error", err)
		}
	default:
		logger.Panicw("You must specify a valid `mode` in config file.")
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
		logger.Panicw("Error starting "+module.Name()+".", "Error", err)
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
			logger.Info("Got stop signal, stopping " + module.Name() + ", ...")
			module.Stop()
			return
		case <-statsTicker:
			metrics := module.GatherStats()
			statsClient.SendMetrics(metrics)
		}
	}
}
