package messaging

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// RabbitMQConfig holds RabbitMQ connection configuration
type RabbitMQConfig struct {
	URL          string
	ExchangeName string
	ExchangeType string // topic, direct, fanout, headers
	Durable      bool
	AutoDelete   bool
}

// RabbitMQPublisher handles publishing messages to RabbitMQ
type RabbitMQPublisher struct {
	conn         *amqp.Connection
	channel      *amqp.Channel
	exchangeName string
}

// Message represents a message to be published
type Message struct {
	RoutingKey  string
	ContentType string
	Body        interface{}
}

// NewRabbitMQPublisher creates a new RabbitMQ publisher with connection management
func NewRabbitMQPublisher(cfg RabbitMQConfig) (*RabbitMQPublisher, error) {
	// Set defaults
	if cfg.ExchangeType == "" {
		cfg.ExchangeType = "topic"
	}

	// Connect to RabbitMQ
	conn, err := amqp.Dial(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to RabbitMQ: %w", err)
	}

	// Create channel
	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to open channel: %w", err)
	}

	// Declare exchange
	err = ch.ExchangeDeclare(
		cfg.ExchangeName, // name
		cfg.ExchangeType, // type
		cfg.Durable,      // durable
		cfg.AutoDelete,   // auto-deleted
		false,            // internal
		false,            // no-wait
		nil,              // arguments
	)
	if err != nil {
		ch.Close()
		conn.Close()
		return nil, fmt.Errorf("failed to declare exchange: %w", err)
	}

	publisher := &RabbitMQPublisher{
		conn:         conn,
		channel:      ch,
		exchangeName: cfg.ExchangeName,
	}

	// Setup connection close handler
	go publisher.handleConnectionErrors()

	return publisher, nil
}

// Publish publishes a message to the exchange
func (p *RabbitMQPublisher) Publish(ctx context.Context, msg Message) error {
	// Marshal body to JSON if not already bytes
	var body []byte
	var err error

	switch v := msg.Body.(type) {
	case []byte:
		body = v
	case string:
		body = []byte(v)
	default:
		body, err = json.Marshal(msg.Body)
		if err != nil {
			return fmt.Errorf("failed to marshal message body: %w", err)
		}
	}

	// Set default content type
	contentType := msg.ContentType
	if contentType == "" {
		contentType = "application/json"
	}

	// Publish with context
	err = p.channel.PublishWithContext(
		ctx,
		p.exchangeName, // exchange
		msg.RoutingKey, // routing key
		false,          // mandatory
		false,          // immediate
		amqp.Publishing{
			ContentType:  contentType,
			Body:         body,
			DeliveryMode: amqp.Persistent, // Make messages persistent
			Timestamp:    time.Now(),
		},
	)

	if err != nil {
		return fmt.Errorf("failed to publish message: %w", err)
	}

	log.Printf("Published message to exchange '%s' with routing key '%s'",
		p.exchangeName, msg.RoutingKey)

	return nil
}

// PublishJSON is a convenience method for publishing JSON messages
func (p *RabbitMQPublisher) PublishJSON(ctx context.Context, routingKey string, data interface{}) error {
	return p.Publish(ctx, Message{
		RoutingKey:  routingKey,
		ContentType: "application/json",
		Body:        data,
	})
}

// Close closes the channel and connection
func (p *RabbitMQPublisher) Close() error {
	if p.channel != nil {
		if err := p.channel.Close(); err != nil {
			return fmt.Errorf("failed to close channel: %w", err)
		}
	}

	if p.conn != nil {
		if err := p.conn.Close(); err != nil {
			return fmt.Errorf("failed to close connection: %w", err)
		}
	}

	return nil
}

// handleConnectionErrors monitors connection errors
func (p *RabbitMQPublisher) handleConnectionErrors() {
	errChan := make(chan *amqp.Error)
	p.conn.NotifyClose(errChan)

	err := <-errChan
	if err != nil {
		log.Printf("RabbitMQ connection closed: %v", err)
	}
}

// IsConnected checks if the publisher is still connected
func (p *RabbitMQPublisher) IsConnected() bool {
	return p.conn != nil && !p.conn.IsClosed()
}
