package main

import (
	"flag"
	"fmt"
	"github.com/openmetric/yamf/executor"
	"github.com/openmetric/yamf/internal/logging"
	"github.com/openmetric/yamf/internal/utils"
	"go.uber.org/zap"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	configFile := flag.String("config", "", "Path to the `config file`.")
	flag.Parse()

	var err error
	var module *executor.Executor
	var logger *zap.SugaredLogger

	config := struct {
		Executor *executor.Config `yaml:"executor"`
		Log      *logging.Config  `yaml:"log"`
	}{
		Executor: executor.NewConfig(),
		Log:      logging.NewConfig(),
	}
	if err := utils.UnmarshalYAMLFile(*configFile, config); err != nil {
		panic(fmt.Sprintf("Error reading config file: %s", err))
	}

	if logger, err = logging.NewLogger(config.Log); err != nil {
		panic(fmt.Sprintf("Error initializing logger: %s", err))
	}

	if module, err = executor.NewExecutor(config.Executor, logger); err != nil {
		logger.Panicw("Error initializing executor.", "Error", err)
	}

	if err = module.Start(); err != nil {
		logger.Panicw("Error starting executor.", "Error", err)
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)

	for {
		select {
		case <-c:
			logger.Info("Got stop signal, stopping executor...")
			module.Stop()
			return
		}
	}
}
