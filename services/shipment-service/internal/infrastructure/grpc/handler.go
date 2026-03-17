package grpc

import (
	"context"
	"errors"
	"log"

	"vektor-shipment/services/shipment-service/internal/domain"
	"vektor-shipment/services/shipment-service/internal/service"
	"vektor-shipment/shared/proto/shipment"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ShipmentHandler implements the gRPC ShipmentService interface
type ShipmentHandler struct {
	shipment.UnimplementedShipmentServiceServer
	service *service.ShipmentService
}

// NewShipmentHandler creates a new gRPC handler
func NewShipmentHandler(svc *service.ShipmentService) *ShipmentHandler {
	return &ShipmentHandler{
		service: svc,
	}
}

// CreateShipment creates a new shipment with initial "pending" status
func (h *ShipmentHandler) CreateShipment(ctx context.Context, req *shipment.CreateShipmentRequest) (*shipment.CreateShipmentResponse, error) {
	log.Printf("CreateShipment called for reference: %s", req.ReferenceNumber)

	// Validate request
	if err := validateCreateShipmentRequest(req); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid request: %v", err)
	}

	// Map proto to domain
	input := service.CreateShipmentInput{
		ReferenceNumber: req.ReferenceNumber,
		Origin:          req.Origin,
		Destination:     req.Destination,
		Driver: domain.DriverDetails{
			DriverID:    req.Driver.DriverId,
			DriverName:  req.Driver.DriverName,
			DriverPhone: req.Driver.DriverPhone,
		},
		Unit: domain.UnitDetails{
			UnitID:      req.Unit.UnitId,
			UnitType:    req.Unit.UnitType,
			PlateNumber: req.Unit.PlateNumber,
		},
		ShipmentAmount: req.ShipmentAmount,
		DriverRevenue:  req.DriverRevenue,
	}

	// Call service
	result, err := h.service.CreateShipment(ctx, input)
	if err != nil {
		if errors.Is(err, domain.ErrDuplicateReference) {
			return nil, status.Errorf(codes.AlreadyExists, "shipment with reference number %s already exists", req.ReferenceNumber)
		}
		log.Printf("Error creating shipment: %v", err)
		return nil, status.Errorf(codes.Internal, "failed to create shipment: %v", err)
	}

	return &shipment.CreateShipmentResponse{
		Shipment: result.ToProto(),
		Message:  "Shipment created successfully",
	}, nil
}

// GetShipment retrieves shipment details by reference number
func (h *ShipmentHandler) GetShipment(ctx context.Context, req *shipment.GetShipmentRequest) (*shipment.GetShipmentResponse, error) {
	log.Printf("GetShipment called for reference: %s", req.ReferenceNumber)

	if req.ReferenceNumber == "" {
		return nil, status.Error(codes.InvalidArgument, "reference number is required")
	}

	result, err := h.service.GetShipment(ctx, req.ReferenceNumber)
	if err != nil {
		if errors.Is(err, domain.ErrShipmentNotFound) {
			return nil, status.Errorf(codes.NotFound, "shipment not found: %s", req.ReferenceNumber)
		}
		log.Printf("Error getting shipment: %v", err)
		return nil, status.Errorf(codes.Internal, "failed to get shipment: %v", err)
	}

	return &shipment.GetShipmentResponse{
		Shipment: result.ToProto(),
	}, nil
}

// AddStatusEvent adds a new status change event to the shipment
func (h *ShipmentHandler) AddStatusEvent(ctx context.Context, req *shipment.AddStatusEventRequest) (*shipment.AddStatusEventResponse, error) {
	log.Printf("AddStatusEvent called for reference: %s, new status: %s", req.ReferenceNumber, req.NewStatus.String())

	// Validate request
	if err := validateAddStatusEventRequest(req); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid request: %v", err)
	}

	// Map proto to domain
	input := service.AddStatusEventInput{
		ReferenceNumber: req.ReferenceNumber,
		NewStatus:       domain.FromProtoStatus(req.NewStatus),
		Description:     req.Description,
		Location:        req.Location,
		RecordedBy:      req.RecordedBy,
	}

	// Call service
	event, updatedShipment, err := h.service.AddStatusEvent(ctx, input)
	if err != nil {
		if errors.Is(err, domain.ErrShipmentNotFound) {
			return nil, status.Errorf(codes.NotFound, "shipment not found: %s", req.ReferenceNumber)
		}
		if errors.Is(err, domain.ErrInvalidStatusTransition) {
			return nil, status.Errorf(codes.FailedPrecondition, "invalid status transition: %v", err)
		}
		log.Printf("Error adding status event: %v", err)
		return nil, status.Errorf(codes.Internal, "failed to add status event: %v", err)
	}

	return &shipment.AddStatusEventResponse{
		Event:           event.ToProto(),
		UpdatedShipment: updatedShipment.ToProto(),
		Message:         "Status event added successfully",
	}, nil
}

// GetShipmentHistory retrieves all status change events for a shipment
func (h *ShipmentHandler) GetShipmentHistory(ctx context.Context, req *shipment.GetShipmentHistoryRequest) (*shipment.GetShipmentHistoryResponse, error) {
	log.Printf("GetShipmentHistory called for reference: %s", req.ReferenceNumber)

	if req.ReferenceNumber == "" {
		return nil, status.Error(codes.InvalidArgument, "reference number is required")
	}

	events, err := h.service.GetShipmentHistory(ctx, req.ReferenceNumber)
	if err != nil {
		if errors.Is(err, domain.ErrShipmentNotFound) {
			return nil, status.Errorf(codes.NotFound, "shipment not found: %s", req.ReferenceNumber)
		}
		log.Printf("Error getting shipment history: %v", err)
		return nil, status.Errorf(codes.Internal, "failed to get shipment history: %v", err)
	}

	// Convert domain events to proto
	protoEvents := make([]*shipment.StatusEvent, len(events))
	for i, event := range events {
		protoEvents[i] = event.ToProto()
	}

	return &shipment.GetShipmentHistoryResponse{
		Events: protoEvents,
	}, nil
}

// validateCreateShipmentRequest validates the create shipment request
func validateCreateShipmentRequest(req *shipment.CreateShipmentRequest) error {
	if req.ReferenceNumber == "" {
		return errors.New("reference number is required")
	}
	if req.Origin == "" {
		return errors.New("origin is required")
	}
	if req.Destination == "" {
		return errors.New("destination is required")
	}
	if req.Driver == nil {
		return errors.New("driver details are required")
	}
	if req.Driver.DriverId == "" {
		return errors.New("driver ID is required")
	}
	if req.Unit == nil {
		return errors.New("unit details are required")
	}
	if req.Unit.UnitId == "" {
		return errors.New("unit ID is required")
	}
	if req.ShipmentAmount < 0 {
		return errors.New("shipment amount cannot be negative")
	}
	if req.DriverRevenue < 0 {
		return errors.New("driver revenue cannot be negative")
	}
	return nil
}

// validateAddStatusEventRequest validates the add status event request
func validateAddStatusEventRequest(req *shipment.AddStatusEventRequest) error {
	if req.ReferenceNumber == "" {
		return errors.New("reference number is required")
	}
	if req.NewStatus == shipment.ShipmentStatus_STATUS_UNSPECIFIED {
		return errors.New("status is required")
	}
	if req.RecordedBy == "" {
		return errors.New("recorded_by is required")
	}
	return nil
}
