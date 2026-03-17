package domain

import (
	"time"

	pb "vektor-shipment/shared/proto/shipment"

	"google.golang.org/protobuf/types/known/timestamppb"
)

// StatusEvent represents a shipment status change event
type StatusEvent struct {
	ID          string
	ShipmentID  string
	Status      ShipmentStatus
	Description string
	Location    string
	OccurredAt  time.Time
	RecordedBy  string
}

// NewStatusEvent creates a new status event
func NewStatusEvent(
	id, shipmentID string,
	status ShipmentStatus,
	description, location, recordedBy string,
) *StatusEvent {
	return &StatusEvent{
		ID:          id,
		ShipmentID:  shipmentID,
		Status:      status,
		Description: description,
		Location:    location,
		OccurredAt:  time.Now(),
		RecordedBy:  recordedBy,
	}
}

// ToProto converts domain status event to proto status event
func (e *StatusEvent) ToProto() *pb.StatusEvent {
	return &pb.StatusEvent{
		Id:          e.ID,
		ShipmentId:  e.ShipmentID,
		Status:      e.Status.ToProto(),
		Description: e.Description,
		Location:    e.Location,
		OccurredAt:  timestampProto(e.OccurredAt),
		RecordedBy:  e.RecordedBy,
	}
}

// timestampProto converts time.Time to protobuf Timestamp
func timestampProto(t time.Time) *timestamppb.Timestamp {
	return timestamppb.New(t)
}
