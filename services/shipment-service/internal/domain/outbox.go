package domain

import (
	"context"
	"encoding/json"
	"time"
)

// OutboxEvent represents an event stored in the outbox table for reliable delivery
type OutboxEvent struct {
	ID           string
	AggregateID  string // The shipment ID
	EventType    string // e.g., "shipment.created", "shipment.status_changed"
	Payload      json.RawMessage
	RoutingKey   string
	Status       OutboxStatus
	Attempts     int
	LastAttempt  *time.Time
	ErrorMessage string
	CreatedAt    time.Time
	ProcessedAt  *time.Time
}

// OutboxStatus represents the processing status of an outbox event
type OutboxStatus string

const (
	OutboxStatusPending   OutboxStatus = "pending"
	OutboxStatusProcessed OutboxStatus = "processed"
	OutboxStatusFailed    OutboxStatus = "failed"
)

// ShipmentCreatedEvent represents the payload for shipment.created event
type ShipmentCreatedEvent struct {
	ShipmentID      string    `json:"shipment_id"`
	ReferenceNumber string    `json:"reference_number"`
	Origin          string    `json:"origin"`
	Destination     string    `json:"destination"`
	Status          string    `json:"status"`
	DriverID        string    `json:"driver_id"`
	DriverName      string    `json:"driver_name"`
	UnitID          string    `json:"unit_id"`
	ShipmentAmount  float64   `json:"shipment_amount"`
	DriverRevenue   float64   `json:"driver_revenue"`
	Timestamp       time.Time `json:"timestamp"`
}

// ShipmentStatusChangedEvent represents the payload for shipment.status_changed event
type ShipmentStatusChangedEvent struct {
	ShipmentID      string    `json:"shipment_id"`
	ReferenceNumber string    `json:"reference_number"`
	OldStatus       string    `json:"old_status"`
	NewStatus       string    `json:"new_status"`
	Location        string    `json:"location"`
	Description     string    `json:"description"`
	RecordedBy      string    `json:"recorded_by"`
	Timestamp       time.Time `json:"timestamp"`
}

// OutboxRepository defines operations for the transactional outbox pattern
type OutboxRepository interface {
	// SaveOutboxEvent saves an event to the outbox table (used in same transaction as domain operation)
	SaveOutboxEvent(ctx context.Context, event *OutboxEvent) error

	// GetPendingEvents retrieves events that need to be published (status=pending, attempts < max)
	GetPendingEvents(ctx context.Context, limit int) ([]*OutboxEvent, error)

	// MarkAsProcessed marks an event as successfully processed
	MarkAsProcessed(ctx context.Context, eventID string) error

	// MarkAsFailed marks an event as failed after max retries
	MarkAsFailed(ctx context.Context, eventID string, errorMsg string) error

	// IncrementAttempt increments the attempt counter for an event
	IncrementAttempt(ctx context.Context, eventID string, errorMsg string) error
}
