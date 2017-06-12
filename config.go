package main

import (
	"fmt"
	"github.com/openmetric/yamf/executor"
	"github.com/openmetric/yamf/logging"
	"github.com/openmetric/yamf/scheduler"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"os"
)

// main config of the program
type Config struct {
	Log       *logging.LoggerConfig `yaml:"log"` // global logger
	Scheduler *scheduler.Config     `yaml:"scheduler"`
	Executor  *executor.Config      `yaml:"executor"`
}

// NewConfig returns a default option values
func NewConfig() *Config {
	config := &Config{
		Log: logging.NewLoggerConfig(),

		Scheduler: &scheduler.Config{
			ListenAddr:      ":8080",
			DBPath:          "./var/db",
			HTTPLogFilename: "./var/log/http.log",
			Log:             logging.NewLoggerConfig(),
		},

		Executor: &executor.Config{
			NumWorkers: 4,
			Log:        logging.NewLoggerConfig(),
		},
	}

	return config
}

func loadConfig(configFile string) *Config {
	if configFile == "" {
		fmt.Println("Missing required option `-config /path/to/config.yml`")
		os.Exit(1)
	}

	configContent, err := ioutil.ReadFile(configFile)
	if err != nil {
		fmt.Println("Error reading config file:", err)
		os.Exit(1)
	}

	config := NewConfig()
	err = yaml.Unmarshal(configContent, config)
	if err != nil {
		fmt.Println("Error reading config file:", err)
		os.Exit(1)
	}

	return config
}
