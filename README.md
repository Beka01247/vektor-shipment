# Vektor Shipment - Transportation Management System

A production-ready microservices platform for managing shipment logistics with guaranteed event delivery and real-time status tracking.

## Overview

This system implements a **Transactional Outbox Pattern** for reliable event publishing, ensuring **At-Least-Once Delivery** guarantees. All shipment state changes are atomically persisted to the database alongside outbox events, which are asynchronously published to RabbitMQ by a background worker.

### Key Features

- ✅ **Transactional Outbox Pattern** - No lost events, atomic database + message broker operations
- ✅ **State Machine Enforcement** - Valid status transitions with business rule validation
- ✅ **Event-Driven Architecture** - RabbitMQ topic exchange with durable queues
- ✅ **Production-Grade Reliability** - Retry logic, error tracking, graceful shutdown
- ✅ **Clean Architecture** - Domain-driven design with comprehensive test coverage

## Quick Start

### Prerequisites

- **Go 1.23+**
- **Kubernetes** - Choose one:
  - **Option 1: Minikube**

    ```bash
    # Install Minikube
    brew install minikube

    # Start Minikube cluster
    minikube start --driver=docker --memory=4096 --cpus=2

    # Enable ingress (optional)
    minikube addons enable ingress

    # Verify Minikube is running
    kubectl get nodes
    ```

  - **Option 2: Docker Desktop**

- **Tilt** - Install via: `brew install tilt-dev/tap/tilt`
- **grpcurl** (for testing) - Install via: `brew install grpcurl`

### Running the Service

1. **Clone the repository**:

   ```bash
   git clone <repository-url>
   cd vektor-shipment
   ```

2. **Start all services with Tilt**:

   ```bash
   tilt up
   ```

   **Note**: If using Minikube, ensure your kubectl context is set correctly:

   ```bash
   # Check current context
   kubectl config current-context

   # Switch to Minikube (if needed)
   kubectl config use-context minikube

   # Switch to Docker Desktop (if needed)
   kubectl config use-context docker-desktop
   ```

   This will:
   - Build Docker images for all services
   - Deploy to local Kubernetes cluster
   - Start PostgreSQL and RabbitMQ
   - Initialize database schemas
   - Setup RabbitMQ topology (exchanges, queues, bindings)
   - Start outbox worker for guaranteed event delivery

3. **Monitor service health**:
   - **Tilt UI**: http://localhost:10350 - Real-time logs and status
   - **RabbitMQ Management**: http://localhost:15672 (guest/guest)
   - **Shipment gRPC Service**: localhost:50052

4. **Test the service**:

   ```bash
   # Create a shipment
   grpcurl -plaintext -d '{
     "reference_number": "SHP001",
     "origin": "New York, NY",
     "destination": "Los Angeles, CA",
     "driver": {"driver_id": "DRV-001", "driver_name": "John Doe"},
     "unit": {"unit_id": "VEH-001"},
     "shipment_amount": 1250.00,
     "driver_revenue": 875.00
   }' localhost:50052 shipment.ShipmentService/CreateShipment

   # Get shipment details
   grpcurl -plaintext -d '{"reference_number": "SHP001"}' \
     localhost:50052 shipment.ShipmentService/GetShipment
   ```

5. **Enable debug consumer** (optional - to see events being consumed):
   - Set `ENABLE_DEBUG_CONSUMER=true` in [main.go](services/shipment-service/cmd/main.go) line 37
   - Restart service (Tilt will auto-reload)
   - Watch service logs to see event consumption in real-time

### Running Tests

```bash
# Run all tests
cd services/shipment-service
go test ./... -v

# Run tests with coverage
go test ./... -cover -coverprofile=coverage.out

# View coverage report
go tool cover -html=coverage.out

# Run specific test
go test ./internal/service -run TestCreateShipment -v
```

**Test Coverage**: Unit tests for domain logic, service layer, and repository layer. Integration tests verify transactional outbox pattern behavior.

## Architecture

### System Architecture

This project follows **Clean Architecture** (Hexagonal/Ports & Adapters) with an **Event-Driven** approach:

```
┌─────────────────────────────────────────────────────────────────┐
│                    Infrastructure Layer                         │
│  gRPC Server │ PostgreSQL │ RabbitMQ │ Outbox Worker            │
└───────────────────────────┬─────────────────────────────────────┘
                            │ depends on ↓
┌───────────────────────────┴───────────────────────────────────────┐
│                    Application Layer                              │
│  Service Orchestration │ Transaction Management │ Event Publishing│
└───────────────────────────┬───────────────────────────────────────┘
                            │ depends on ↓
┌───────────────────────────┴──────────────────────────────────────┐
│                      Domain Layer                                │
│  Shipment Entity │ Status Transitions │ Business Rules           │
└──────────────────────────────────────────────────────────────────┘
```

### Messaging Architecture (Transactional Outbox Pattern)

```
┌─────────────┐     ┌──────────────┐     ┌────────────┐     ┌──────────────┐
│   Client    │────▶│   Service    │────▶│ PostgreSQL │     │   RabbitMQ   │
│   (gRPC)    │     │              │     │            │     │   Exchange   │
└─────────────┘     └──────────────┘     └────────────┘     └──────────────┘
                           │                    │                    ▲
                           │                    │                    │
                           │          ┌─────────▼────────┐           │
                           │          │  outbox_events   │           │
                           │          │  (pending/       │           │
                           │          │   processed/     │           │
                           │          │   failed)        │           │
                           │          └─────────┬────────┘           │
                           │                    │                    │
                           └────────────────────┼────────────────────┘
                                   ┌────────────▼──────────┐
                                   │   Outbox Worker       │
                                   │   (polls every 5s)    │
                                   │   • Retry logic (5x)  │
                                   │   • Error tracking    │
                                   └───────────────────────┘
```

**Flow**:

1. Client calls gRPC CreateShipment/AddStatusEvent
2. Service saves shipment + outbox event **atomically** (single transaction)
3. Background worker polls `outbox_events` table for pending events
4. Worker publishes events to RabbitMQ topic exchange
5. On success → mark event as `processed`; On failure → retry (max 5 attempts)
6. Consumers receive events from bound queues

### Shipment State Flow

```
┌─────────┐     ┌───────────┐     ┌────────────┐     ┌───────────┐
│ PENDING │────▶│ PICKED_UP │────▶│ IN_TRANSIT │────▶│ DELIVERED │
└─────────┘     └───────────┘     └────────────┘     └───────────┘
                                          │
                                          ▼
                                   ┌──────────────┐
                                   │  CANCELLED   │
                                   └──────────────┘
```

**Valid Transitions**:

- `PENDING` → `PICKED_UP` (driver picks up shipment)
- `PICKED_UP` → `IN_TRANSIT` (shipment en route)
- `IN_TRANSIT` → `DELIVERED` (successful delivery - terminal state)
- `IN_TRANSIT` → `CANCELLED` (cancellation - terminal state)

**Error Conditions**:

- ❌ Invalid transition (e.g., PENDING → DELIVERED) → `invalid status transition`
- ❌ Modifying terminal state (DELIVERED/CANCELLED) → `cannot modify terminal state`
- ❌ Duplicate reference number → `duplicate reference number`
- ❌ Shipment not found → `shipment not found`

### RabbitMQ Topology

**Exchange**: `shipment-events` (type: topic, durable)

**Queues**:

- `shipment-events-all` → routing key: `shipment.#` (catch-all)
- `shipment-created-queue` → routing key: `shipment.created`
- `shipment-status-changed-queue` → routing key: `shipment.status_changed`
- `shipment-debug` → routing key: `#` (auto-delete, for debugging)

### Debug Consumer

**`ENABLE_DEBUG_CONSUMER` Configuration**:

- **`true`** (development): Starts a consumer that listens to `shipment-debug` queue and logs all events in formatted output. Useful for:
  - Verifying events are published correctly
  - Debugging event payloads
  - Demonstrating end-to-end message flow
  - Testing consumer acknowledgments

- **`false`** (production - default): Debug consumer disabled. Only application-specific consumers run. Messages remain in queues until consumed by downstream services.

## Design Decisions

### Why Transactional Outbox Pattern?

**Problem**: Traditional "fire and forget" event publishing can lose messages if:

- Message broker is temporarily unavailable
- Application crashes after DB commit but before event publish
- Network issues prevent message delivery

**Solution**: Transactional Outbox Pattern guarantees **At-Least-Once Delivery**:

1. Events are saved to `outbox_events` table in **same transaction** as domain changes
2. Background worker asynchronously publishes events from outbox table
3. Retry logic (5 attempts) handles transient failures
4. No events lost even during system failures

### Why RabbitMQ?

- **Topic Exchange**: Flexible routing with wildcard patterns (`shipment.#`, `shipment.created`)
- **Durability**: Persistent queues survive broker restarts
- **Acknowledgments**: Manual acks ensure messages aren't lost on consumer failure
- **Mature Ecosystem**: Production-proven, excellent monitoring (Management UI)
- **Performance**: Handles high throughput with low latency

### Why PostgreSQL?

- **ACID Transactions**: Required for transactional outbox atomicity
- **JSONB Support**: Flexible event payload storage without schema migrations
- **Reliability**: Production-proven data integrity and consistency
- **Query Performance**: Efficient indexing for outbox polling and shipment lookups

### Repository Pattern

- **Abstraction**: Easy to swap database implementations (PostgreSQL → MySQL)
- **Testability**: Mock repositories for unit tests without real database
- **Single Responsibility**: Repository handles data access, service handles business logic
- **Interface Segregation**: Clean contracts between layers

## Assumptions

1. **Unique Reference Numbers**: Each shipment has a unique reference number (enforced by unique index)
2. **Sequential Status Transitions**: Business rules require following valid state machine paths
3. **Terminal States**: DELIVERED and CANCELLED states cannot be modified once set
4. **Event Order**: Events are published in order they were created (FIFO from outbox table)
5. **Idempotency**: Consumers should handle duplicate events gracefully (at-least-once delivery)
6. **Network Reliability**: PostgreSQL and RabbitMQ connections may fail transiently (retry logic implemented)
7. **Clock Synchronization**: Server timestamps are used for event ordering

## Technology Stack

- **Backend**: Go 1.23
- **Communication**: gRPC (Protocol Buffers)
- **Database**: PostgreSQL 16
- **Message Broker**: RabbitMQ 3.12
- **Container Orchestration**: Kubernetes
- **Development Tools**: Tilt (hot reload), grpcurl (testing)
- **Testing**: Go testing package, table-driven tests

## Project Structure

```
vektor-shipment/
├── services/
│   └── shipment-service/
│       ├── cmd/main.go                    # Application entry point
│       └── internal/
│           ├── domain/                    # Business logic & entities
│           │   ├── shipment.go           # Shipment entity & rules
│           │   ├── status_event.go       # Status tracking
│           │   └── outbox.go             # Outbox events
│           ├── service/                   # Use cases & orchestration
│           │   ├── service.go            # Business logic layer
│           │   └── service_test.go       # Unit tests
│           └── infrastructure/           # External dependencies
│               ├── grpc/                 # gRPC handlers
│               ├── repository/           # PostgreSQL implementation
│               ├── worker/               # Outbox background worker
│               ├── messaging/            # RabbitMQ topology setup
│               └── consumer/             # Debug event consumer
├── shared/                               # Shared libraries (DRY)
│   ├── proto/shipment/                  # gRPC definitions
│   ├── db/                              # Database utilities
│   ├── messaging/                       # RabbitMQ client
│   └── env/                             # Config management
└── infra/                                # Infrastructure configs
    ├── development/                      # Local K8s manifests
    └── production/                       # Production configs
```

## Additional Documentation

- 📖 [TESTING.md](./TESTING.md) - Comprehensive testing scenarios with grpcurl commands

## Development Workflow

### Minikube Management (if using Minikube)

```bash
# Start Minikube
minikube start

# Stop Minikube (preserves cluster state)
minikube stop

# Delete Minikube cluster (clean slate)
minikube delete

# View Minikube dashboard
minikube dashboard

# Access RabbitMQ Management UI (when using Minikube)
minikube service rabbitmq -n infrastructure
```

### Generating Protocol Buffers

```bash
make generate-proto
```

### Building Services Manually

Tilt handles automatic building during development. For manual builds:

```bash
cd services/shipment-service
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o ../../build/shipment-service ./cmd/main.go
```

---
