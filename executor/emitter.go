package executor

import (
	"encoding/json"
	"fmt"
	"github.com/nsqio/go-nsq"
	"github.com/openmetric/yamf/internal/types"
	"os"
)

// Emitter pushes event out
type Emitter interface {
	Emit(*types.Event)
	Close()
}

type NSQEmitter struct {
	NSQDTcpAddr string
	NSQTopic    string

	producer *nsq.Producer
}

func NewNSQEmitter(addr string, topic string) *NSQEmitter {
	nsqdConfig := nsq.NewConfig()
	producer, err := nsq.NewProducer(addr, nsqdConfig)
	if err != nil {
		fmt.Println("error initializing nsqd producer for emitting:", err)
		os.Exit(1)
	}
	return &NSQEmitter{
		NSQDTcpAddr: addr,
		NSQTopic:    topic,
		producer:    producer,
	}
}

func (e *NSQEmitter) Emit(event *types.Event) {
	data, _ := json.Marshal(event)
	e.producer.Publish(e.NSQTopic, data)
}

func (e *NSQEmitter) Close() {
	if e.producer != nil {
		e.producer.Stop()
		e.producer = nil
	}
}

type RabbitMQEmitter struct {
}

func (e *RabbitMQEmitter) Emit(event *types.Event) {

}

func (e *RabbitMQEmitter) Close() {

}

type FileEmitter struct {
	Filename string

	file *os.File
}

func NewFileEmitter(filename string) *FileEmitter {
	file, err := os.OpenFile(filename, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0644)
	if err != nil {
		fmt.Println("Failed to openen file for emitting:", err)
		os.Exit(1)
	}
	return &FileEmitter{
		Filename: filename,
		file:     file,
	}
}

func (e *FileEmitter) Emit(event *types.Event) {
	data, _ := json.Marshal(event)
	e.file.Write(data)
	e.file.Write([]byte("\n"))
}

func (e *FileEmitter) Close() {
	if e.file != nil {
		e.file.Close()
		e.file = nil
	}
}
