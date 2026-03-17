package service

import (
	"context"
	"errors"
	"testing"

	"vektor-shipment/services/shipment-service/internal/domain"
)

// MockRepository implements domain.ShipmentRepository for testing
type MockRepository struct {
	shipments   map[string]*domain.Shipment
	events      map[string][]*domain.StatusEvent
	createError error
	getError    error
	updateError error
	eventError  error
}

func NewMockRepository() *MockRepository {
	return &MockRepository{
		shipments: make(map[string]*domain.Shipment),
		events:    make(map[string][]*domain.StatusEvent),
	}
}

func (m *MockRepository) Create(ctx context.Context, shipment *domain.Shipment) error {
	if m.createError != nil {
		return m.createError
	}
	// Check for duplicate reference number
	for _, s := range m.shipments {
		if s.ReferenceNumber == shipment.ReferenceNumber {
			return domain.ErrDuplicateReference
		}
	}
	m.shipments[shipment.ID] = shipment
	return nil
}

func (m *MockRepository) GetByReferenceNumber(ctx context.Context, refNumber string) (*domain.Shipment, error) {
	if m.getError != nil {
		return nil, m.getError
	}
	for _, s := range m.shipments {
		if s.ReferenceNumber == refNumber {
			return s, nil
		}
	}
	return nil, domain.ErrShipmentNotFound
}

func (m *MockRepository) Update(ctx context.Context, shipment *domain.Shipment) error {
	if m.updateError != nil {
		return m.updateError
	}
	if _, exists := m.shipments[shipment.ID]; !exists {
		return domain.ErrShipmentNotFound
	}
	m.shipments[shipment.ID] = shipment
	return nil
}

func (m *MockRepository) AddStatusEvent(ctx context.Context, event *domain.StatusEvent) error {
	if m.eventError != nil {
		return m.eventError
	}
	m.events[event.ShipmentID] = append(m.events[event.ShipmentID], event)
	return nil
}

func (m *MockRepository) GetStatusEvents(ctx context.Context, shipmentID string) ([]*domain.StatusEvent, error) {
	if m.eventError != nil {
		return nil, m.eventError
	}
	return m.events[shipmentID], nil
}

// GetByID retrieves a shipment by ID (needed for AddStatusEvent)
func (m *MockRepository) GetByID(ctx context.Context, id string) (*domain.Shipment, error) {
	if m.getError != nil {
		return nil, m.getError
	}
	if shipment, exists := m.shipments[id]; exists {
		return shipment, nil
	}
	return nil, domain.ErrShipmentNotFound
}

// OutboxRepository interface implementation for testing transactional outbox pattern
func (m *MockRepository) CreateWithOutbox(ctx context.Context, shipment *domain.Shipment, outboxEvent *domain.OutboxEvent) error {
	// In tests, just create the shipment (outbox events are tested separately)
	return m.Create(ctx, shipment)
}

func (m *MockRepository) UpdateWithOutbox(ctx context.Context, shipment *domain.Shipment, statusEvent *domain.StatusEvent, outboxEvent *domain.OutboxEvent) error {
	// In tests, add status event and update shipment
	if err := m.AddStatusEvent(ctx, statusEvent); err != nil {
		return err
	}
	return m.Update(ctx, shipment)
}

// MockPublisher implements EventPublisher for testing
type MockPublisher struct {
	createdEvents       []*domain.Shipment
	statusChangedEvents []*domain.StatusEvent
	publishError        error
}

func NewMockPublisher() *MockPublisher {
	return &MockPublisher{
		createdEvents:       make([]*domain.Shipment, 0),
		statusChangedEvents: make([]*domain.StatusEvent, 0),
	}
}

func (m *MockPublisher) PublishShipmentCreated(ctx context.Context, shipment *domain.Shipment) error {
	if m.publishError != nil {
		return m.publishError
	}
	m.createdEvents = append(m.createdEvents, shipment)
	return nil
}

func (m *MockPublisher) PublishShipmentStatusChanged(ctx context.Context, shipment *domain.Shipment, event *domain.StatusEvent) error {
	if m.publishError != nil {
		return m.publishError
	}
	m.statusChangedEvents = append(m.statusChangedEvents, event)
	return nil
}

func TestCreateShipment(t *testing.T) {
	repo := NewMockRepository()
	svc := NewShipmentService(repo, repo)

	input := CreateShipmentInput{
		ReferenceNumber: "REF-001",
		Origin:          "New York",
		Destination:     "Los Angeles",
		Driver: domain.DriverDetails{
			DriverID:    "driver-123",
			DriverName:  "John Doe",
			DriverPhone: "+1234567890",
		},
		Unit: domain.UnitDetails{
			UnitID:      "unit-456",
			UnitType:    "Truck",
			PlateNumber: "ABC123",
		},
		ShipmentAmount: 1000.0,
		DriverRevenue:  800.0,
	}

	shipment, err := svc.CreateShipment(context.Background(), input)
	if err != nil {
		t.Fatalf("CreateShipment() error = %v", err)
	}

	if shipment == nil {
		t.Fatal("CreateShipment() returned nil shipment")
	}

	if shipment.ReferenceNumber != input.ReferenceNumber {
		t.Errorf("ReferenceNumber = %v, want %v", shipment.ReferenceNumber, input.ReferenceNumber)
	}

	if shipment.CurrentStatus != domain.StatusPending {
		t.Errorf("CurrentStatus = %v, want %v", shipment.CurrentStatus, domain.StatusPending)
	}

	// Check if shipment was persisted
	retrieved, err := repo.GetByReferenceNumber(context.Background(), input.ReferenceNumber)
	if err != nil {
		t.Errorf("Shipment not found in repository: %v", err)
	}
	if retrieved.ID != shipment.ID {
		t.Errorf("Retrieved shipment ID = %v, want %v", retrieved.ID, shipment.ID)
	}
}

func TestCreateShipmentDuplicateReference(t *testing.T) {
	repo := NewMockRepository()
	svc := NewShipmentService(repo, repo)

	input := CreateShipmentInput{
		ReferenceNumber: "REF-001",
		Origin:          "New York",
		Destination:     "Los Angeles",
		Driver: domain.DriverDetails{
			DriverID: "driver-123",
		},
		Unit: domain.UnitDetails{
			UnitID: "unit-456",
		},
		ShipmentAmount: 1000.0,
		DriverRevenue:  800.0,
	}

	// Create first shipment
	_, err := svc.CreateShipment(context.Background(), input)
	if err != nil {
		t.Fatalf("First CreateShipment() error = %v", err)
	}

	// Try to create duplicate
	_, err = svc.CreateShipment(context.Background(), input)
	if err == nil {
		t.Fatal("CreateShipment() expected duplicate error but got nil")
	}
	if !errors.Is(err, domain.ErrDuplicateReference) {
		t.Errorf("CreateShipment() error = %v, want %v", err, domain.ErrDuplicateReference)
	}
}

func TestGetShipment(t *testing.T) {
	repo := NewMockRepository()
	svc := NewShipmentService(repo, repo)

	// Create a shipment first
	input := CreateShipmentInput{
		ReferenceNumber: "REF-001",
		Origin:          "New York",
		Destination:     "Los Angeles",
		Driver: domain.DriverDetails{
			DriverID: "driver-123",
		},
		Unit: domain.UnitDetails{
			UnitID: "unit-456",
		},
		ShipmentAmount: 1000.0,
		DriverRevenue:  800.0,
	}

	created, _ := svc.CreateShipment(context.Background(), input)

	// Retrieve the shipment
	retrieved, err := svc.GetShipment(context.Background(), "REF-001")
	if err != nil {
		t.Fatalf("GetShipment() error = %v", err)
	}

	if retrieved.ID != created.ID {
		t.Errorf("GetShipment() ID = %v, want %v", retrieved.ID, created.ID)
	}
}

func TestGetShipmentNotFound(t *testing.T) {
	repo := NewMockRepository()
	svc := NewShipmentService(repo, repo)

	_, err := svc.GetShipment(context.Background(), "NON-EXISTENT")
	if err == nil {
		t.Fatal("GetShipment() expected error but got nil")
	}
	if !errors.Is(err, domain.ErrShipmentNotFound) {
		t.Errorf("GetShipment() error = %v, want %v", err, domain.ErrShipmentNotFound)
	}
}

func TestAddStatusEvent(t *testing.T) {
	repo := NewMockRepository()
	svc := NewShipmentService(repo, repo)

	// Create a shipment first
	createInput := CreateShipmentInput{
		ReferenceNumber: "REF-001",
		Origin:          "New York",
		Destination:     "Los Angeles",
		Driver: domain.DriverDetails{
			DriverID: "driver-123",
		},
		Unit: domain.UnitDetails{
			UnitID: "unit-456",
		},
		ShipmentAmount: 1000.0,
		DriverRevenue:  800.0,
	}

	shipment, _ := svc.CreateShipment(context.Background(), createInput)

	// Add a valid status event (PENDING -> PICKED_UP)
	eventInput := AddStatusEventInput{
		ReferenceNumber: "REF-001",
		NewStatus:       domain.StatusPickedUp,
		Description:     "Driver picked up the shipment",
		Location:        "123 Main St",
		RecordedBy:      "driver-123",
	}

	event, updatedShipment, err := svc.AddStatusEvent(context.Background(), eventInput)
	if err != nil {
		t.Fatalf("AddStatusEvent() error = %v", err)
	}

	if event == nil {
		t.Fatal("AddStatusEvent() returned nil event")
	}

	if updatedShipment.CurrentStatus != domain.StatusPickedUp {
		t.Errorf("UpdatedShipment.CurrentStatus = %v, want %v", updatedShipment.CurrentStatus, domain.StatusPickedUp)
	}

	if event.Status != domain.StatusPickedUp {
		t.Errorf("Event.Status = %v, want %v", event.Status, domain.StatusPickedUp)
	}

	// Verify shipment was updated in repository
	retrieved, _ := repo.GetByReferenceNumber(context.Background(), "REF-001")
	if retrieved.CurrentStatus != domain.StatusPickedUp {
		t.Errorf("Repository shipment status = %v, want %v", retrieved.CurrentStatus, domain.StatusPickedUp)
	}

	// Verify initial event exists
	if len(repo.events[shipment.ID]) < 2 {
		t.Errorf("Expected at least 2 events (initial + pickup), got %d", len(repo.events[shipment.ID]))
	}
}

func TestAddStatusEventInvalidTransition(t *testing.T) {
	repo := NewMockRepository()
	svc := NewShipmentService(repo, repo)

	// Create a shipment
	createInput := CreateShipmentInput{
		ReferenceNumber: "REF-001",
		Origin:          "New York",
		Destination:     "Los Angeles",
		Driver: domain.DriverDetails{
			DriverID: "driver-123",
		},
		Unit: domain.UnitDetails{
			UnitID: "unit-456",
		},
		ShipmentAmount: 1000.0,
		DriverRevenue:  800.0,
	}

	svc.CreateShipment(context.Background(), createInput)

	// Try to add an invalid status event (PENDING -> DELIVERED, skipping intermediate steps)
	eventInput := AddStatusEventInput{
		ReferenceNumber: "REF-001",
		NewStatus:       domain.StatusDelivered,
		Description:     "Trying to deliver without pickup",
		Location:        "456 Elm St",
		RecordedBy:      "system",
	}

	_, _, err := svc.AddStatusEvent(context.Background(), eventInput)
	if err == nil {
		t.Fatal("AddStatusEvent() expected error for invalid transition but got nil")
	}

	if !errors.Is(err, domain.ErrInvalidStatusTransition) {
		t.Errorf("AddStatusEvent() error = %v, want %v", err, domain.ErrInvalidStatusTransition)
	}
}

func TestGetShipmentHistory(t *testing.T) {
	repo := NewMockRepository()
	svc := NewShipmentService(repo, repo)

	// Create a shipment
	createInput := CreateShipmentInput{
		ReferenceNumber: "REF-001",
		Origin:          "New York",
		Destination:     "Los Angeles",
		Driver: domain.DriverDetails{
			DriverID: "driver-123",
		},
		Unit: domain.UnitDetails{
			UnitID: "unit-456",
		},
		ShipmentAmount: 1000.0,
		DriverRevenue:  800.0,
	}

	svc.CreateShipment(context.Background(), createInput)

	// Add a status event
	svc.AddStatusEvent(context.Background(), AddStatusEventInput{
		ReferenceNumber: "REF-001",
		NewStatus:       domain.StatusPickedUp,
		Description:     "Picked up",
		Location:        "Origin",
		RecordedBy:      "driver-123",
	})

	// Get history
	events, err := svc.GetShipmentHistory(context.Background(), "REF-001")
	if err != nil {
		t.Fatalf("GetShipmentHistory() error = %v", err)
	}

	if len(events) < 2 {
		t.Errorf("GetShipmentHistory() returned %d events, want at least 2", len(events))
	}
}

func TestCompleteShipmentLifecycle(t *testing.T) {
	repo := NewMockRepository()
	svc := NewShipmentService(repo, repo)

	// Create shipment
	shipment, _ := svc.CreateShipment(context.Background(), CreateShipmentInput{
		ReferenceNumber: "REF-LIFECYCLE",
		Origin:          "Origin",
		Destination:     "Destination",
		Driver: domain.DriverDetails{
			DriverID: "driver-123",
		},
		Unit: domain.UnitDetails{
			UnitID: "unit-456",
		},
		ShipmentAmount: 1000.0,
		DriverRevenue:  800.0,
	})

	if shipment.CurrentStatus != domain.StatusPending {
		t.Errorf("Initial status = %v, want %v", shipment.CurrentStatus, domain.StatusPending)
	}

	// PENDING -> PICKED_UP
	_, shipment, _ = svc.AddStatusEvent(context.Background(), AddStatusEventInput{
		ReferenceNumber: "REF-LIFECYCLE",
		NewStatus:       domain.StatusPickedUp,
		Description:     "Picked up",
		Location:        "Origin",
		RecordedBy:      "driver-123",
	})

	if shipment.CurrentStatus != domain.StatusPickedUp {
		t.Errorf("After pickup status = %v, want %v", shipment.CurrentStatus, domain.StatusPickedUp)
	}

	// PICKED_UP -> IN_TRANSIT
	_, shipment, _ = svc.AddStatusEvent(context.Background(), AddStatusEventInput{
		ReferenceNumber: "REF-LIFECYCLE",
		NewStatus:       domain.StatusInTransit,
		Description:     "In transit",
		Location:        "Highway",
		RecordedBy:      "driver-123",
	})

	if shipment.CurrentStatus != domain.StatusInTransit {
		t.Errorf("After transit status = %v, want %v", shipment.CurrentStatus, domain.StatusInTransit)
	}

	// IN_TRANSIT -> DELIVERED
	_, shipment, _ = svc.AddStatusEvent(context.Background(), AddStatusEventInput{
		ReferenceNumber: "REF-LIFECYCLE",
		NewStatus:       domain.StatusDelivered,
		Description:     "Delivered",
		Location:        "Destination",
		RecordedBy:      "driver-123",
	})

	if shipment.CurrentStatus != domain.StatusDelivered {
		t.Errorf("Final status = %v, want %v", shipment.CurrentStatus, domain.StatusDelivered)
	}

	// Verify we cannot change status from terminal state
	_, _, err := svc.AddStatusEvent(context.Background(), AddStatusEventInput{
		ReferenceNumber: "REF-LIFECYCLE",
		NewStatus:       domain.StatusInTransit,
		Description:     "Try to change terminal state",
		Location:        "Somewhere",
		RecordedBy:      "system",
	})

	if err == nil {
		t.Error("Expected error when trying to change from terminal state")
	}

	// Verify history
	events, _ := svc.GetShipmentHistory(context.Background(), "REF-LIFECYCLE")
	if len(events) != 4 { // initial pending + 3 transitions
		t.Errorf("History length = %d, want 4", len(events))
	}
}
