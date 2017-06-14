package logging

import (
	"fmt"
	"github.com/op/go-logging"
	"os"
	"strings"
	"sync"
)

// common logging setup shared by scheduler and executors

const defaultFormat string = `[%{time:15:04:05.000}][%{module}][%{level}] %{message}`

var fileManager = newLogFileManager()

// Logger wraps logging.Logger so that other code does not need to import op/go-logging
type Logger struct {
	logging.Logger
}

// LoggerConfig is a logger section in configuration file
type LoggerConfig struct {
	Filename string `yaml:"filename"`
	Level    string `yaml:"level"`

	// default format is `[%{time:15:04:05.000}][%{module}][%{level}] %{message}`
	Format string `yaml:"format"`
}

func NewLoggerConfig() *LoggerConfig {
	return &LoggerConfig{
		Filename: "/dev/stdout",
		Level:    "info",
		Format:   defaultFormat,
	}
}

type logFileManager struct {
	opened map[string]*os.File
	sync.RWMutex
}

func newLogFileManager() *logFileManager {
	return &logFileManager{
		opened: make(map[string]*os.File),
	}
}

func (m *logFileManager) OpenFile(filename string) (*os.File, error) {
	m.Lock()
	defer m.Unlock()
	if f, ok := m.opened[filename]; ok {
		return f, nil
	}
	file, err := os.OpenFile(filename, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0644)
	if err != nil {
		return nil, err
	}
	m.opened[filename] = file
	return file, nil
}

// GetLogger creates a logger
func GetLogger(module string, config *LoggerConfig) *Logger {
	format := config.Format
	if format == "" {
		format = defaultFormat
	}
	formatter := logging.MustStringFormatter(format)

	logFile, err := fileManager.OpenFile(config.Filename)
	if err != nil {
		fmt.Println("Failed to open logfile:", err)
		os.Exit(1)
	}

	backend := logging.NewBackendFormatter(logging.NewLogBackend(logFile, "", 0), formatter)

	var level logging.Level
	switch strings.ToLower(config.Level) {
	case "critical":
		level = logging.CRITICAL
	case "error":
		level = logging.ERROR
	case "warning":
		level = logging.WARNING
	case "notice":
		level = logging.NOTICE
	case "info":
		level = logging.INFO
	case "debug":
		level = logging.DEBUG
	default:
		fmt.Println("Invalid log level:", config.Level)
		os.Exit(1)
	}
	leveledBackend := logging.AddModuleLevel(backend)
	leveledBackend.SetLevel(level, module)

	logger := logging.MustGetLogger(module)
	logger.SetBackend(leveledBackend)
	return &Logger{*logger}
}
