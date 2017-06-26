package executor

import (
	"encoding/json"
	"fmt"
	"github.com/nsqio/go-nsq"
	"github.com/openmetric/yamf/internal/types"
	"github.com/streadway/amqp"
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
	Uri       string
	QueueName string

	conn *amqp.Connection
	ch   *amqp.Channel
	q    *amqp.Queue
}

func NewRabbitMQEmitter(uri string, queueName string) *RabbitMQEmitter {
	conn, err := amqp.Dial(uri)
	if err != nil {
		fmt.Println("error initializing rabbitmq producer for emitting:", err)
		os.Exit(1)
	}
	ch, err := conn.Channel()
	if err != nil {
		fmt.Println("failed to open rabbitmq channel")
		os.Exit(1)
	}
	q, err := ch.QueueDeclare(queueName, true, false, false, false, nil)
	if err != nil {
		fmt.Println("failed to declare queue")
		os.Exit(1)
	}

	e := &RabbitMQEmitter{
		Uri:       uri,
		QueueName: queueName,
		conn:      conn,
		ch:        ch,
		q:         &q,
	}
	return e
}

func (e *RabbitMQEmitter) Emit(event *types.Event) {
	data, _ := json.Marshal(event)
	e.ch.Publish("", e.q.Name, false, false, amqp.Publishing{
		DeliveryMode: amqp.Persistent,
		ContentType:  "application/json",
		Body:         data,
	})
}

func (e *RabbitMQEmitter) Close() {
	if e.conn != nil {
		e.ch.Close()
		e.conn.Close()
		e.ch = nil
		e.conn = nil
		e.q = nil
	}
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
	out := fmt.Sprintf("[%v][%d] %s\n", event.Timestamp, event.Status, event.Identifier)
	e.file.WriteString(out)
}

func (e *FileEmitter) Close() {
	if e.file != nil {
		e.file.Close()
		e.file = nil
	}
}

func NewEmitter(config *EmitConfig) (Emitter, error) {
	var emitter Emitter
	switch config.Type {
	case "file":
		emitter = NewFileEmitter(config.Filename)
	case "nsq":
		emitter = NewNSQEmitter(config.NSQDTCPAddr, config.NSQTopic)
	case "rabbitmq":
		emitter = NewRabbitMQEmitter(config.RabbitMQUri, config.RabbitMQQueue)
	default:
		return nil, fmt.Errorf("unsupported emit type: %s", config.Type)
	}
	return emitter, nil
}
