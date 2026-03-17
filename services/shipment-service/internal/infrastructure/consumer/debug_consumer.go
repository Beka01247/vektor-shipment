package consumer

import (
	"context"
	"fmt"
	"log"

	amqp "github.com/rabbitmq/amqp091-go"
)

// DebugConsumer consumes messages from RabbitMQ for debugging/demonstration
type DebugConsumer struct {
	conn       *amqp.Connection
	channel    *amqp.Channel
	queueName  string
	stopCh     chan struct{}
	deliveries <-chan amqp.Delivery
}

// NewDebugConsumer creates a new debug consumer
func NewDebugConsumer(url string, queueName string) (*DebugConsumer, error) {
	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to RabbitMQ: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to open channel: %w", err)
	}

	// Set QoS to process one message at a time
	err = ch.Qos(
		1,     // prefetch count
		0,     // prefetch size
		false, // global
	)
	if err != nil {
		ch.Close()
		conn.Close()
		return nil, fmt.Errorf("failed to set QoS: %w", err)
	}

	// Start consuming
	deliveries, err := ch.Consume(
		queueName, // queue
		"",        // consumer tag (auto-generated)
		false,     // auto-ack (manual ack for reliability)
		false,     // exclusive
		false,     // no-local
		false,     // no-wait
		nil,       // args
	)
	if err != nil {
		ch.Close()
		conn.Close()
		return nil, fmt.Errorf("failed to start consumer: %w", err)
	}

	return &DebugConsumer{
		conn:       conn,
		channel:    ch,
		queueName:  queueName,
		stopCh:     make(chan struct{}),
		deliveries: deliveries,
	}, nil
}

// Start begins consuming messages
func (c *DebugConsumer) Start(ctx context.Context) {
	log.Printf("[DebugConsumer] Started consuming from queue: %s", c.queueName)

	for {
		select {
		case msg, ok := <-c.deliveries:
			if !ok {
				log.Println("[DebugConsumer] Delivery channel closed")
				return
			}

			c.handleMessage(msg)

		case <-c.stopCh:
			log.Println("[DebugConsumer] Stopped")
			return

		case <-ctx.Done():
			log.Println("[DebugConsumer] Context cancelled, stopping...")
			return
		}
	}
}

// handleMessage processes a single message
func (c *DebugConsumer) handleMessage(msg amqp.Delivery) {
	log.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	log.Printf("[DebugConsumer] Received Event")
	log.Printf("  Exchange:     %s", msg.Exchange)
	log.Printf("  Routing Key:  %s", msg.RoutingKey)
	log.Printf("  Content Type: %s", msg.ContentType)
	log.Printf("  Timestamp:    %s", msg.Timestamp)
	log.Printf("  Message ID:   %s", msg.MessageId)
	log.Printf("  Payload:")
	log.Printf("%s", string(msg.Body))
	log.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	// Acknowledge the message
	if err := msg.Ack(false); err != nil {
		log.Printf("[DebugConsumer] Failed to ack message: %v", err)
	}
}

// Stop stops the consumer
func (c *DebugConsumer) Stop() error {
	close(c.stopCh)

	if c.channel != nil {
		if err := c.channel.Close(); err != nil {
			return fmt.Errorf("failed to close channel: %w", err)
		}
	}

	if c.conn != nil {
		if err := c.conn.Close(); err != nil {
			return fmt.Errorf("failed to close connection: %w", err)
		}
	}

	return nil
}
