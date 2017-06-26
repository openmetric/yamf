package logging

import (
	"go.uber.org/zap"
)

type Config struct {
	OutputPaths []string `yaml:"output_paths"`
	Level       string   `yaml:"level"`
	Encoding    string   `yaml:"encoding"`
}

func NewConfig() *Config {
	return &Config{
		OutputPaths: []string{"stdout"},
		Level:       "info",
		Encoding:    "console",
	}
}

func NewLogger(config *Config) (*zap.SugaredLogger, error) {
	cfg := zap.NewProductionConfig()

	cfg.OutputPaths = config.OutputPaths
	cfg.DisableCaller = true
	cfg.DisableStacktrace = true
	cfg.Encoding = config.Encoding

	lvl := zap.AtomicLevel{}
	if err := lvl.UnmarshalText([]byte(config.Level)); err != nil {
		return nil, err
	}
	cfg.Level = lvl

	if logger, err := cfg.Build(); err != nil {
		return nil, err
	} else {
		return logger.Sugar(), nil
	}
}
