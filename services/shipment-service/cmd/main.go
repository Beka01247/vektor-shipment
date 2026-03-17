package main

import (
	"context"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"vektor-shipment/services/shipment-service/internal/infrastructure/consumer"
	"vektor-shipment/services/shipment-service/internal/infrastructure/grpc"
	"vektor-shipment/services/shipment-service/internal/infrastructure/messaging"
	"vektor-shipment/services/shipment-service/internal/infrastructure/repository"
	"vektor-shipment/services/shipment-service/internal/infrastructure/worker"
	"vektor-shipment/services/shipment-service/internal/service"
	"vektor-shipment/shared/db"
	"vektor-shipment/shared/env"
	sharedMessaging "vektor-shipment/shared/messaging"
	pb "vektor-shipment/shared/proto/shipment"

	grpcLib "google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

var (
	grpcAddr            = env.GetString("GRPC_ADDR", ":50052")
	dbHost              = env.GetString("DB_HOST", "localhost")
	dbPort              = env.GetString("DB_PORT", "5432")
	dbUser              = env.GetString("DB_USER", "shipment_user")
	dbPassword          = env.GetString("DB_PASSWORD", "shipment_pass")
	dbName              = env.GetString("DB_NAME", "shipment_db")
	amqpURL             = env.GetString("AMQP_URL", "amqp://guest:guest@localhost:5672/")
	outboxInterval      = env.GetInt("OUTBOX_INTERVAL_SECONDS", 5)
	outboxBatchSize     = env.GetInt("OUTBOX_BATCH_SIZE", 10)
	enableDebugConsumer = env.GetBool("ENABLE_DEBUG_CONSUMER", false)
	debugConsumerQueue  = env.GetString("DEBUG_CONSUMER_QUEUE", "shipment-debug")
)

func main() {
	log.Println("Starting Shipment Service...")

	log.Printf("Connecting to database at %s:%s...", dbHost, dbPort)
	database, err := db.NewPostgresConnection(db.PostgresConfig{
		Host:     dbHost,
		Port:     dbPort,
		User:     dbUser,
		Password: dbPassword,
		DBName:   dbName,
		SSLMode:  "disable",
	})
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.CloseConnection(database)
	log.Println("Database connection established")

	// Init database schema
	repo := repository.NewPostgresRepository(database)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := repo.InitSchema(ctx); err != nil {
		log.Fatalf("Failed to initialize schema: %v", err)
	}
	log.Println("Database schema initialized")

	// Init RabbitMQ publisher using shared messaging
	log.Printf("Connecting to RabbitMQ at %s...", amqpURL)
	rabbitPublisher, err := sharedMessaging.NewRabbitMQPublisher(sharedMessaging.RabbitMQConfig{
		URL:          amqpURL,
		ExchangeName: "shipment-events",
		ExchangeType: "topic",
		Durable:      true,
		AutoDelete:   false,
	})
	if err != nil {
		log.Fatalf("Failed to initialize RabbitMQ publisher: %v", err)
	}
	defer rabbitPublisher.Close()
	log.Println("RabbitMQ publisher initialized")

	// Setup RabbitMQ topology (exchanges, queues, bindings)
	log.Println("Setting up RabbitMQ infrastructure topology...")
	topology := messaging.DefaultShipmentTopology()
	if err := messaging.SetupTopology(amqpURL, topology); err != nil {
		log.Fatalf("Failed to setup RabbitMQ topology: %v", err)
	}
	log.Printf("RabbitMQ topology setup complete: %d queues configured", len(topology.Queues))

	// Start outbox worker for guaranteed event delivery
	log.Printf("Starting outbox worker (interval: %ds, batch size: %d)...", outboxInterval, outboxBatchSize)
	outboxWorker := worker.NewOutboxWorker(
		repo,
		rabbitPublisher,
		time.Duration(outboxInterval)*time.Second,
		outboxBatchSize,
	)
	workerCtx, workerCancel := context.WithCancel(context.Background())
	defer workerCancel()

	go outboxWorker.Start(workerCtx)
	log.Println("Outbox worker started - polling for pending events")

	// Optionally start debug consumer
	var debugConsumer *consumer.DebugConsumer
	var consumerCancel context.CancelFunc
	if enableDebugConsumer {
		log.Printf("Starting debug consumer on queue '%s'...", debugConsumerQueue)
		debugConsumer, err = consumer.NewDebugConsumer(amqpURL, debugConsumerQueue)
		if err != nil {
			log.Printf("Warning: failed to initialize debug consumer: %v", err)
		} else {
			consumerCtx, cancel := context.WithCancel(context.Background())
			consumerCancel = cancel

			go debugConsumer.Start(consumerCtx)
			log.Println("Debug consumer started")
		}
	}

	// Initialize service with transactional outbox repository
	shipmentService := service.NewShipmentService(repo, repo)
	log.Println("Shipment service initialized with transactional outbox pattern")

	// Initialize gRPC server
	grpcServer := grpcLib.NewServer()
	handler := grpc.NewShipmentHandler(shipmentService)
	pb.RegisterShipmentServiceServer(grpcServer, handler)

	// Enable reflection for grpcurl
	reflection.Register(grpcServer)

	// Start gRPC server
	listener, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		log.Fatalf("Failed to listen on %s: %v", grpcAddr, err)
	}

	log.Printf("Shipment Service gRPC server listening on %s", grpcAddr)

	// Start server
	go func() {
		if err := grpcServer.Serve(listener); err != nil {
			log.Fatalf("Failed to serve: %v", err)
		}
	}()

	// Gracefully shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	// Stop outbox worker
	log.Println("Stopping outbox worker...")
	workerCancel()

	// Stop debug consumer if running
	if debugConsumer != nil && consumerCancel != nil {
		log.Println("Stopping debug consumer...")
		consumerCancel()
		if err := debugConsumer.Stop(); err != nil {
			log.Printf("Error stopping debug consumer: %v", err)
		}
	}

	// Stop gRPC server
	grpcServer.GracefulStop()
	log.Println("Server stopped")
}
