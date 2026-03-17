package messaging

import (
	"fmt"
	"log"

	amqp "github.com/rabbitmq/amqp091-go"
)

// TopologyConfig defines the RabbitMQ infrastructure topology
type TopologyConfig struct {
	ExchangeName string
	ExchangeType string // topic, direct, fanout
	Queues       []QueueConfig
}

// QueueConfig defines a queue and its bindings
type QueueConfig struct {
	Name        string
	Durable     bool
	AutoDelete  bool
	RoutingKeys []string // Routing keys to bind to the exchange
}

// SetupTopology declares exchanges, queues, and bindings on RabbitMQ
func SetupTopology(url string, config TopologyConfig) error {
	// Connect to RabbitMQ
	conn, err := amqp.Dial(url)
	if err != nil {
		return fmt.Errorf("failed to connect to RabbitMQ: %w", err)
	}
	defer conn.Close()

	// Create channel
	ch, err := conn.Channel()
	if err != nil {
		return fmt.Errorf("failed to open channel: %w", err)
	}
	defer ch.Close()

	// Declare exchange
	log.Printf("[Topology] Declaring exchange: %s (type=%s)", config.ExchangeName, config.ExchangeType)
	err = ch.ExchangeDeclare(
		config.ExchangeName, // name
		config.ExchangeType, // type
		true,                // durable
		false,               // auto-deleted
		false,               // internal
		false,               // no-wait
		nil,                 // arguments
	)
	if err != nil {
		return fmt.Errorf("failed to declare exchange: %w", err)
	}

	// Declare queues and bindings
	for _, queueCfg := range config.Queues {
		log.Printf("[Topology] Declaring queue: %s", queueCfg.Name)

		_, err := ch.QueueDeclare(
			queueCfg.Name,       // name
			queueCfg.Durable,    // durable
			queueCfg.AutoDelete, // delete when unused
			false,               // exclusive
			false,               // no-wait
			nil,                 // arguments
		)
		if err != nil {
			return fmt.Errorf("failed to declare queue %s: %w", queueCfg.Name, err)
		}

		// Bind queue to exchange with routing keys
		for _, routingKey := range queueCfg.RoutingKeys {
			log.Printf("[Topology] Binding queue %s to exchange %s with routing key: %s",
				queueCfg.Name, config.ExchangeName, routingKey)

			err := ch.QueueBind(
				queueCfg.Name,       // queue name
				routingKey,          // routing key
				config.ExchangeName, // exchange
				false,               // no-wait
				nil,                 // arguments
			)
			if err != nil {
				return fmt.Errorf("failed to bind queue %s: %w", queueCfg.Name, err)
			}
		}
	}

	log.Printf("[Topology] Successfully configured topology for exchange: %s", config.ExchangeName)
	return nil
}

// DefaultShipmentTopology returns the default topology for the shipment service
func DefaultShipmentTopology() TopologyConfig {
	return TopologyConfig{
		ExchangeName: "shipment-events",
		ExchangeType: "topic",
		Queues: []QueueConfig{
			{
				Name:       "shipment-events-all",
				Durable:    true,
				AutoDelete: false,
				RoutingKeys: []string{
					"shipment.#", // Catch all shipment events
				},
			},
			{
				Name:       "shipment-created-queue",
				Durable:    true,
				AutoDelete: false,
				RoutingKeys: []string{
					"shipment.created",
				},
			},
			{
				Name:       "shipment-status-changed-queue",
				Durable:    true,
				AutoDelete: false,
				RoutingKeys: []string{
					"shipment.status_changed",
				},
			},
			{
				Name:       "shipment-debug",
				Durable:    false, // Not durable for debugging
				AutoDelete: true,  // Auto-delete when no consumers
				RoutingKeys: []string{
					"#", // Catch everything for debugging
				},
			},
		},
	}
}
