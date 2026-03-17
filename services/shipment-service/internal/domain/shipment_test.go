package domain

import (
	"testing"
)

func TestNewShipment(t *testing.T) {
	tests := []struct {
		name           string
		refNumber      string
		origin         string
		destination    string
		driver         DriverDetails
		unit           UnitDetails
		shipmentAmount float64
		driverRevenue  float64
		wantErr        bool
		errContains    string
	}{
		{
			name:        "valid shipment",
			refNumber:   "REF-001",
			origin:      "New York",
			destination: "Los Angeles",
			driver: DriverDetails{
				DriverID:    "driver-123",
				DriverName:  "John Doe",
				DriverPhone: "+1234567890",
			},
			unit: UnitDetails{
				UnitID:      "unit-456",
				UnitType:    "Truck",
				PlateNumber: "ABC123",
			},
			shipmentAmount: 1000.0,
			driverRevenue:  800.0,
			wantErr:        false,
		},
		{
			name:        "missing reference number",
			refNumber:   "",
			origin:      "New York",
			destination: "Los Angeles",
			driver: DriverDetails{
				DriverID: "driver-123",
			},
			unit: UnitDetails{
				UnitID: "unit-456",
			},
			wantErr:     true,
			errContains: "reference number is required",
		},
		{
			name:        "missing origin",
			refNumber:   "REF-001",
			origin:      "",
			destination: "Los Angeles",
			driver: DriverDetails{
				DriverID: "driver-123",
			},
			unit: UnitDetails{
				UnitID: "unit-456",
			},
			wantErr:     true,
			errContains: "origin is required",
		},
		{
			name:        "missing destination",
			refNumber:   "REF-001",
			origin:      "New York",
			destination: "",
			driver: DriverDetails{
				DriverID: "driver-123",
			},
			unit: UnitDetails{
				UnitID: "unit-456",
			},
			wantErr:     true,
			errContains: "destination is required",
		},
		{
			name:        "negative shipment amount",
			refNumber:   "REF-001",
			origin:      "New York",
			destination: "Los Angeles",
			driver: DriverDetails{
				DriverID: "driver-123",
			},
			unit: UnitDetails{
				UnitID: "unit-456",
			},
			shipmentAmount: -100.0,
			wantErr:        true,
			errContains:    "shipment amount cannot be negative",
		},
		{
			name:        "negative driver revenue",
			refNumber:   "REF-001",
			origin:      "New York",
			destination: "Los Angeles",
			driver: DriverDetails{
				DriverID: "driver-123",
			},
			unit: UnitDetails{
				UnitID: "unit-456",
			},
			shipmentAmount: 1000.0,
			driverRevenue:  -100.0,
			wantErr:        true,
			errContains:    "driver revenue cannot be negative",
		},
		{
			name:        "missing driver ID",
			refNumber:   "REF-001",
			origin:      "New York",
			destination: "Los Angeles",
			driver: DriverDetails{
				DriverID: "",
			},
			unit: UnitDetails{
				UnitID: "unit-456",
			},
			wantErr:     true,
			errContains: "driver ID is required",
		},
		{
			name:        "missing unit ID",
			refNumber:   "REF-001",
			origin:      "New York",
			destination: "Los Angeles",
			driver: DriverDetails{
				DriverID: "driver-123",
			},
			unit: UnitDetails{
				UnitID: "",
			},
			wantErr:     true,
			errContains: "unit ID is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			shipment, err := NewShipment(
				"test-id",
				tt.refNumber,
				tt.origin,
				tt.destination,
				tt.driver,
				tt.unit,
				tt.shipmentAmount,
				tt.driverRevenue,
			)

			if tt.wantErr {
				if err == nil {
					t.Errorf("NewShipment() expected error but got nil")
				} else if tt.errContains != "" && !contains(err.Error(), tt.errContains) {
					t.Errorf("NewShipment() error = %v, want error containing %v", err, tt.errContains)
				}
			} else {
				if err != nil {
					t.Errorf("NewShipment() unexpected error: %v", err)
				}
				if shipment == nil {
					t.Errorf("NewShipment() returned nil shipment")
				}
				if shipment != nil && shipment.CurrentStatus != StatusPending {
					t.Errorf("NewShipment() status = %v, want %v", shipment.CurrentStatus, StatusPending)
				}
			}
		})
	}
}

func TestShipmentStatusTransitions(t *testing.T) {
	tests := []struct {
		name          string
		currentStatus ShipmentStatus
		newStatus     ShipmentStatus
		shouldAllow   bool
	}{
		// Valid transitions from Pending
		{
			name:          "pending to picked_up",
			currentStatus: StatusPending,
			newStatus:     StatusPickedUp,
			shouldAllow:   true,
		},
		{
			name:          "pending to cancelled",
			currentStatus: StatusPending,
			newStatus:     StatusCancelled,
			shouldAllow:   true,
		},
		// Invalid transitions from Pending
		{
			name:          "pending to in_transit (skip pickup)",
			currentStatus: StatusPending,
			newStatus:     StatusInTransit,
			shouldAllow:   false,
		},
		{
			name:          "pending to delivered (skip pickup and transit)",
			currentStatus: StatusPending,
			newStatus:     StatusDelivered,
			shouldAllow:   false,
		},
		// Valid transitions from PickedUp
		{
			name:          "picked_up to in_transit",
			currentStatus: StatusPickedUp,
			newStatus:     StatusInTransit,
			shouldAllow:   true,
		},
		{
			name:          "picked_up to cancelled",
			currentStatus: StatusPickedUp,
			newStatus:     StatusCancelled,
			shouldAllow:   true,
		},
		{
			name:          "picked_up to failed",
			currentStatus: StatusPickedUp,
			newStatus:     StatusFailed,
			shouldAllow:   true,
		},
		// Invalid transitions from PickedUp
		{
			name:          "picked_up to pending (backwards)",
			currentStatus: StatusPickedUp,
			newStatus:     StatusPending,
			shouldAllow:   false,
		},
		{
			name:          "picked_up to delivered (skip transit)",
			currentStatus: StatusPickedUp,
			newStatus:     StatusDelivered,
			shouldAllow:   false,
		},
		// Valid transitions from InTransit
		{
			name:          "in_transit to delivered",
			currentStatus: StatusInTransit,
			newStatus:     StatusDelivered,
			shouldAllow:   true,
		},
		{
			name:          "in_transit to failed",
			currentStatus: StatusInTransit,
			newStatus:     StatusFailed,
			shouldAllow:   true,
		},
		// Invalid transitions from InTransit
		{
			name:          "in_transit to pending (backwards)",
			currentStatus: StatusInTransit,
			newStatus:     StatusPending,
			shouldAllow:   false,
		},
		{
			name:          "in_transit to picked_up (backwards)",
			currentStatus: StatusInTransit,
			newStatus:     StatusPickedUp,
			shouldAllow:   false,
		},
		{
			name:          "in_transit to cancelled",
			currentStatus: StatusInTransit,
			newStatus:     StatusCancelled,
			shouldAllow:   false,
		},
		// Terminal states - no further transitions
		{
			name:          "delivered to in_transit (terminal state)",
			currentStatus: StatusDelivered,
			newStatus:     StatusInTransit,
			shouldAllow:   false,
		},
		{
			name:          "cancelled to pending (terminal state)",
			currentStatus: StatusCancelled,
			newStatus:     StatusPending,
			shouldAllow:   false,
		},
		{
			name:          "failed to in_transit (terminal state)",
			currentStatus: StatusFailed,
			newStatus:     StatusInTransit,
			shouldAllow:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			shipment := &Shipment{
				ID:              "test-id",
				ReferenceNumber: "REF-001",
				Origin:          "Origin",
				Destination:     "Destination",
				CurrentStatus:   tt.currentStatus,
				Driver: DriverDetails{
					DriverID: "driver-123",
				},
				Unit: UnitDetails{
					UnitID: "unit-456",
				},
			}

			canTransition := shipment.CanTransitionTo(tt.newStatus)
			if canTransition != tt.shouldAllow {
				t.Errorf(
					"CanTransitionTo(%v -> %v) = %v, want %v",
					tt.currentStatus,
					tt.newStatus,
					canTransition,
					tt.shouldAllow,
				)
			}

			// Test UpdateStatus
			err := shipment.UpdateStatus(tt.newStatus)
			if tt.shouldAllow {
				if err != nil {
					t.Errorf("UpdateStatus(%v -> %v) unexpected error: %v", tt.currentStatus, tt.newStatus, err)
				}
				if shipment.CurrentStatus != tt.newStatus {
					t.Errorf("UpdateStatus() did not update status, got %v want %v", shipment.CurrentStatus, tt.newStatus)
				}
			} else {
				if err == nil {
					t.Errorf("UpdateStatus(%v -> %v) expected error but got nil", tt.currentStatus, tt.newStatus)
				}
				if err != ErrInvalidStatusTransition {
					t.Errorf("UpdateStatus() error = %v, want %v", err, ErrInvalidStatusTransition)
				}
			}
		})
	}
}

func TestShipmentTerminalStates(t *testing.T) {
	tests := []struct {
		name       string
		status     ShipmentStatus
		isTerminal bool
	}{
		{
			name:       "pending is not terminal",
			status:     StatusPending,
			isTerminal: false,
		},
		{
			name:       "picked_up is not terminal",
			status:     StatusPickedUp,
			isTerminal: false,
		},
		{
			name:       "in_transit is not terminal",
			status:     StatusInTransit,
			isTerminal: false,
		},
		{
			name:       "delivered is terminal",
			status:     StatusDelivered,
			isTerminal: true,
		},
		{
			name:       "cancelled is terminal",
			status:     StatusCancelled,
			isTerminal: true,
		},
		{
			name:       "failed is terminal",
			status:     StatusFailed,
			isTerminal: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			shipment := &Shipment{
				CurrentStatus: tt.status,
			}

			isTerminal := shipment.IsTerminalState()
			if isTerminal != tt.isTerminal {
				t.Errorf("IsTerminalState() for %v = %v, want %v", tt.status, isTerminal, tt.isTerminal)
			}
		})
	}
}

func TestShipmentStatusString(t *testing.T) {
	tests := []struct {
		status ShipmentStatus
		want   string
	}{
		{StatusPending, "PENDING"},
		{StatusPickedUp, "PICKED_UP"},
		{StatusInTransit, "IN_TRANSIT"},
		{StatusDelivered, "DELIVERED"},
		{StatusCancelled, "CANCELLED"},
		{StatusFailed, "FAILED"},
		{ShipmentStatus(999), "UNKNOWN"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := tt.status.String()
			if got != tt.want {
				t.Errorf("String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNewStatusEvent(t *testing.T) {
	event := NewStatusEvent(
		"event-123",
		"shipment-456",
		StatusPickedUp,
		"Driver picked up the shipment",
		"123 Main St",
		"driver-789",
	)

	if event == nil {
		t.Fatal("NewStatusEvent() returned nil")
	}

	if event.ID != "event-123" {
		t.Errorf("ID = %v, want %v", event.ID, "event-123")
	}
	if event.ShipmentID != "shipment-456" {
		t.Errorf("ShipmentID = %v, want %v", event.ShipmentID, "shipment-456")
	}
	if event.Status != StatusPickedUp {
		t.Errorf("Status = %v, want %v", event.Status, StatusPickedUp)
	}
	if event.Description != "Driver picked up the shipment" {
		t.Errorf("Description = %v, want %v", event.Description, "Driver picked up the shipment")
	}
	if event.Location != "123 Main St" {
		t.Errorf("Location = %v, want %v", event.Location, "123 Main St")
	}
	if event.RecordedBy != "driver-789" {
		t.Errorf("RecordedBy = %v, want %v", event.RecordedBy, "driver-789")
	}
	if event.OccurredAt.IsZero() {
		t.Error("OccurredAt should not be zero")
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
