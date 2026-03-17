package domain

import (
	"context"
	"errors"
	"time"

	pb "vektor-shipment/shared/proto/shipment"
)

var (
	// ErrShipmentNotFound is returned when a shipment doesn't exist
	ErrShipmentNotFound = errors.New("shipment not found")

	// ErrInvalidStatusTransition is returned when a status change is invalid
	ErrInvalidStatusTransition = errors.New("invalid status transition")

	// ErrDuplicateReference is returned when a reference number already exists
	ErrDuplicateReference = errors.New("shipment with this reference number already exists")

	// ErrInvalidShipment is returned when shipment data is invalid
	ErrInvalidShipment = errors.New("invalid shipment data")
)

// ShipmentStatus represents the lifecycle states of a shipment
type ShipmentStatus int32

const (
	StatusPending   ShipmentStatus = 1
	StatusPickedUp  ShipmentStatus = 2
	StatusInTransit ShipmentStatus = 3
	StatusDelivered ShipmentStatus = 4
	StatusCancelled ShipmentStatus = 5
	StatusFailed    ShipmentStatus = 6
)

// String returns the string representation of the status
func (s ShipmentStatus) String() string {
	switch s {
	case StatusPending:
		return "PENDING"
	case StatusPickedUp:
		return "PICKED_UP"
	case StatusInTransit:
		return "IN_TRANSIT"
	case StatusDelivered:
		return "DELIVERED"
	case StatusCancelled:
		return "CANCELLED"
	case StatusFailed:
		return "FAILED"
	default:
		return "UNKNOWN"
	}
}

// ToProto converts domain status to proto status
func (s ShipmentStatus) ToProto() pb.ShipmentStatus {
	return pb.ShipmentStatus(s)
}

// FromProtoStatus converts proto status to domain status
func FromProtoStatus(s pb.ShipmentStatus) ShipmentStatus {
	return ShipmentStatus(s)
}

// DriverDetails contains driver information
type DriverDetails struct {
	DriverID    string
	DriverName  string
	DriverPhone string
}

// UnitDetails contains vehicle/unit information
type UnitDetails struct {
	UnitID      string
	UnitType    string
	PlateNumber string
}

// Shipment represents the main shipment entity with business logic
type Shipment struct {
	ID              string
	ReferenceNumber string
	Origin          string
	Destination     string
	CurrentStatus   ShipmentStatus
	Driver          DriverDetails
	Unit            UnitDetails
	ShipmentAmount  float64
	DriverRevenue   float64
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// NewShipment creates a new shipment with validation
func NewShipment(
	id, refNumber, origin, destination string,
	driver DriverDetails,
	unit UnitDetails,
	shipmentAmount, driverRevenue float64,
) (*Shipment, error) {
	// Validate required fields
	if refNumber == "" {
		return nil, errors.New("reference number is required")
	}
	if origin == "" {
		return nil, errors.New("origin is required")
	}
	if destination == "" {
		return nil, errors.New("destination is required")
	}
	if shipmentAmount < 0 {
		return nil, errors.New("shipment amount cannot be negative")
	}
	if driverRevenue < 0 {
		return nil, errors.New("driver revenue cannot be negative")
	}
	if driver.DriverID == "" {
		return nil, errors.New("driver ID is required")
	}
	if unit.UnitID == "" {
		return nil, errors.New("unit ID is required")
	}

	now := time.Now()
	return &Shipment{
		ID:              id,
		ReferenceNumber: refNumber,
		Origin:          origin,
		Destination:     destination,
		CurrentStatus:   StatusPending, // Always start with pending
		Driver:          driver,
		Unit:            unit,
		ShipmentAmount:  shipmentAmount,
		DriverRevenue:   driverRevenue,
		CreatedAt:       now,
		UpdatedAt:       now,
	}, nil
}

// CanTransitionTo checks if a status transition is valid based on business rules
func (s *Shipment) CanTransitionTo(newStatus ShipmentStatus) bool {
	// Define valid state transitions
	validTransitions := map[ShipmentStatus][]ShipmentStatus{
		StatusPending: {
			StatusPickedUp,  // Driver picks up the shipment
			StatusCancelled, // Can be cancelled before pickup
		},
		StatusPickedUp: {
			StatusInTransit, // Shipment is now on the way
			StatusCancelled, // Can still be cancelled
			StatusFailed,    // Pickup might fail
		},
		StatusInTransit: {
			StatusDelivered, // Successfully delivered
			StatusFailed,    // Delivery failed (address issues, etc.)
		},
		// Terminal states - no further transitions allowed
		StatusDelivered: {},
		StatusCancelled: {},
		StatusFailed:    {},
	}

	allowedStatuses, exists := validTransitions[s.CurrentStatus]
	if !exists {
		return false
	}

	for _, allowed := range allowedStatuses {
		if allowed == newStatus {
			return true
		}
	}

	return false
}

// UpdateStatus updates the shipment status if the transition is valid
func (s *Shipment) UpdateStatus(newStatus ShipmentStatus) error {
	if !s.CanTransitionTo(newStatus) {
		return ErrInvalidStatusTransition
	}

	s.CurrentStatus = newStatus
	s.UpdatedAt = time.Now()
	return nil
}

// IsTerminalState checks if the shipment is in a terminal state
func (s *Shipment) IsTerminalState() bool {
	return s.CurrentStatus == StatusDelivered ||
		s.CurrentStatus == StatusCancelled ||
		s.CurrentStatus == StatusFailed
}

// ToProto converts domain shipment to proto shipment
func (s *Shipment) ToProto() *pb.Shipment {
	return &pb.Shipment{
		Id:              s.ID,
		ReferenceNumber: s.ReferenceNumber,
		Origin:          s.Origin,
		Destination:     s.Destination,
		CurrentStatus:   s.CurrentStatus.ToProto(),
		Driver: &pb.DriverDetails{
			DriverId:    s.Driver.DriverID,
			DriverName:  s.Driver.DriverName,
			DriverPhone: s.Driver.DriverPhone,
		},
		Unit: &pb.UnitDetails{
			UnitId:      s.Unit.UnitID,
			UnitType:    s.Unit.UnitType,
			PlateNumber: s.Unit.PlateNumber,
		},
		ShipmentAmount: s.ShipmentAmount,
		DriverRevenue:  s.DriverRevenue,
		CreatedAt:      timestampProto(s.CreatedAt),
		UpdatedAt:      timestampProto(s.UpdatedAt),
	}
}

// ShipmentRepository defines the interface for shipment persistence
// This is the port that will be implemented by infrastructure layer
type ShipmentRepository interface {
	// Create creates a new shipment
	Create(ctx context.Context, shipment *Shipment) error

	// GetByReferenceNumber retrieves a shipment by its reference number
	GetByReferenceNumber(ctx context.Context, refNumber string) (*Shipment, error)

	// Update updates an existing shipment
	Update(ctx context.Context, shipment *Shipment) error

	// AddStatusEvent adds a new status change event
	AddStatusEvent(ctx context.Context, event *StatusEvent) error

	// GetStatusEvents retrieves all status events for a shipment
	GetStatusEvents(ctx context.Context, shipmentID string) ([]*StatusEvent, error)
}
