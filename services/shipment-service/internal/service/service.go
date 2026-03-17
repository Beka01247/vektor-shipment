package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"vektor-shipment/services/shipment-service/internal/domain"

	"github.com/google/uuid"
)

// OutboxRepository defines operations for transactional outbox
type OutboxRepository interface {
	CreateWithOutbox(ctx context.Context, shipment *domain.Shipment, outboxEvent *domain.OutboxEvent) error
	UpdateWithOutbox(ctx context.Context, shipment *domain.Shipment, statusEvent *domain.StatusEvent, outboxEvent *domain.OutboxEvent) error
}

// ShipmentService implements the business use cases for shipment management
type ShipmentService struct {
	repo       domain.ShipmentRepository
	outboxRepo OutboxRepository
}

// NewShipmentService creates a new shipment service instance
func NewShipmentService(repo domain.ShipmentRepository, outboxRepo OutboxRepository) *ShipmentService {
	return &ShipmentService{
		repo:       repo,
		outboxRepo: outboxRepo,
	}
}

// CreateShipmentInput contains the data needed to create a shipment
type CreateShipmentInput struct {
	ReferenceNumber string
	Origin          string
	Destination     string
	Driver          domain.DriverDetails
	Unit            domain.UnitDetails
	ShipmentAmount  float64
	DriverRevenue   float64
}

// CreateShipment creates a new shipment with initial "pending" status
func (s *ShipmentService) CreateShipment(ctx context.Context, input CreateShipmentInput) (*domain.Shipment, error) {
	// Generate unique ID
	id := uuid.New().String()

	// Create shipment entity with validation
	shipment, err := domain.NewShipment(
		id,
		input.ReferenceNumber,
		input.Origin,
		input.Destination,
		input.Driver,
		input.Unit,
		input.ShipmentAmount,
		input.DriverRevenue,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create shipment: %w", err)
	}

	// Create event payload
	eventPayload := domain.ShipmentCreatedEvent{
		ShipmentID:      shipment.ID,
		ReferenceNumber: shipment.ReferenceNumber,
		Origin:          shipment.Origin,
		Destination:     shipment.Destination,
		Status:          shipment.CurrentStatus.String(),
		DriverID:        shipment.Driver.DriverID,
		DriverName:      shipment.Driver.DriverName,
		UnitID:          shipment.Unit.UnitID,
		ShipmentAmount:  shipment.ShipmentAmount,
		DriverRevenue:   shipment.DriverRevenue,
		Timestamp:       shipment.CreatedAt,
	}

	payloadBytes, err := json.Marshal(eventPayload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal event payload: %w", err)
	}

	// Create outbox event for guaranteed delivery
	outboxEvent := &domain.OutboxEvent{
		ID:          uuid.New().String(),
		AggregateID: shipment.ID,
		EventType:   "shipment.created",
		Payload:     payloadBytes,
		RoutingKey:  "shipment.created",
		Status:      domain.OutboxStatusPending,
		Attempts:    0,
		CreatedAt:   time.Now(),
	}

	// Save shipment and outbox event atomically (transactional outbox pattern)
	if err := s.outboxRepo.CreateWithOutbox(ctx, shipment, outboxEvent); err != nil {
		if errors.Is(err, domain.ErrDuplicateReference) {
			return nil, err
		}
		return nil, fmt.Errorf("failed to save shipment with outbox: %w", err)
	}

	// Create initial status event (in separate operation, not critical)
	initialEvent := domain.NewStatusEvent(
		uuid.New().String(),
		shipment.ID,
		domain.StatusPending,
		"Shipment created",
		input.Origin,
		"system",
	)

	if err := s.repo.AddStatusEvent(ctx, initialEvent); err != nil {
		log.Printf("Warning: failed to create initial status event: %v", err)
	}

	log.Printf("[Service] Created shipment %s - event queued in outbox for guaranteed delivery", shipment.ReferenceNumber)

	return shipment, nil
}

// GetShipment retrieves a shipment by its reference number
func (s *ShipmentService) GetShipment(ctx context.Context, referenceNumber string) (*domain.Shipment, error) {
	if referenceNumber == "" {
		return nil, errors.New("reference number is required")
	}

	shipment, err := s.repo.GetByReferenceNumber(ctx, referenceNumber)
	if err != nil {
		return nil, err
	}

	return shipment, nil
}

// AddStatusEventInput contains the data needed to add a status event
type AddStatusEventInput struct {
	ReferenceNumber string
	NewStatus       domain.ShipmentStatus
	Description     string
	Location        string
	RecordedBy      string
}

// AddStatusEvent adds a new status change event to the shipment
func (s *ShipmentService) AddStatusEvent(ctx context.Context, input AddStatusEventInput) (*domain.StatusEvent, *domain.Shipment, error) {
	// Retrieve the shipment
	shipment, err := s.repo.GetByReferenceNumber(ctx, input.ReferenceNumber)
	if err != nil {
		return nil, nil, err
	}

	// Validate status transition
	if !shipment.CanTransitionTo(input.NewStatus) {
		return nil, nil, fmt.Errorf(
			"%w: cannot transition from %s to %s",
			domain.ErrInvalidStatusTransition,
			shipment.CurrentStatus.String(),
			input.NewStatus.String(),
		)
	}

	// Update shipment status
	if err := shipment.UpdateStatus(input.NewStatus); err != nil {
		return nil, nil, err
	}

	// Create status event
	event := domain.NewStatusEvent(
		uuid.New().String(),
		shipment.ID,
		input.NewStatus,
		input.Description,
		input.Location,
		input.RecordedBy,
	)

	// Create event payload for messaging
	eventPayload := domain.ShipmentStatusChangedEvent{
		ShipmentID:      shipment.ID,
		ReferenceNumber: shipment.ReferenceNumber,
		OldStatus:       shipment.CurrentStatus.String(),
		NewStatus:       input.NewStatus.String(),
		Location:        input.Location,
		Description:     input.Description,
		RecordedBy:      input.RecordedBy,
		Timestamp:       event.OccurredAt,
	}

	payloadBytes, err := json.Marshal(eventPayload)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal event payload: %w", err)
	}

	// Create outbox event
	outboxEvent := &domain.OutboxEvent{
		ID:          uuid.New().String(),
		AggregateID: shipment.ID,
		EventType:   "shipment.status_changed",
		Payload:     payloadBytes,
		RoutingKey:  "shipment.status_changed",
		Status:      domain.OutboxStatusPending,
		Attempts:    0,
		CreatedAt:   time.Now(),
	}

	// Update shipment, add status event, and save outbox event atomically
	if err := s.outboxRepo.UpdateWithOutbox(ctx, shipment, event, outboxEvent); err != nil {
		return nil, nil, fmt.Errorf("failed to update shipment with outbox: %w", err)
	}

	log.Printf("[Service] Updated shipment %s to status %s - event queued in outbox", shipment.ReferenceNumber, input.NewStatus)

	return event, shipment, nil
}

// GetShipmentHistory retrieves all status change events for a shipment
func (s *ShipmentService) GetShipmentHistory(ctx context.Context, referenceNumber string) ([]*domain.StatusEvent, error) {
	if referenceNumber == "" {
		return nil, errors.New("reference number is required")
	}

	// Get shipment to validate it exists
	shipment, err := s.repo.GetByReferenceNumber(ctx, referenceNumber)
	if err != nil {
		return nil, err
	}

	// Get all events for this shipment
	events, err := s.repo.GetStatusEvents(ctx, shipment.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve status events: %w", err)
	}

	return events, nil
}
