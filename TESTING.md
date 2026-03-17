# Testing Guide - Shipment Service

This guide provides step-by-step commands to test all functionality of the shipment service.

## Prerequisites

```bash
# Install grpcurl if you don't have it
brew install grpcurl

# Make sure Tilt is running
tilt up
```

## Test Scenarios

### 1. Create a Shipment

Create a new shipment from New York to Los Angeles:

```bash
grpcurl -plaintext -d '{
  "reference_number": "SHP001",
  "origin": "New York, NY",
  "destination": "Los Angeles, CA",
  "driver": {
    "driver_id": "DRV-001",
    "driver_name": "John Doe",
    "driver_phone": "+1-555-0123"
  },
  "unit": {
    "unit_id": "VEH-001",
    "unit_type": "Semi-Truck",
    "plate_number": "NY-12345"
  },
  "shipment_amount": 1250.00,
  "driver_revenue": 875.00
}' localhost:50052 shipment.ShipmentService/CreateShipment
```

**Expected Response:**

```json
{
  "shipment": {
    "id": "uuid-here",
    "referenceNumber": "SHP001",
    "origin": "New York, NY",
    "destination": "Los Angeles, CA",
    "currentStatus": "STATUS_PENDING",
    "driver": {
      "driverId": "DRV-001",
      "driverName": "John Doe",
      "driverPhone": "+1-555-0123"
    },
    "unit": {
      "unitId": "VEH-001",
      "unitType": "Semi-Truck",
      "plateNumber": "NY-12345"
    },
    "shipmentAmount": 1250,
    "driverRevenue": 875,
    "createdAt": "2026-03-17T...",
    "updatedAt": "2026-03-17T..."
  },
  "message": "Shipment created successfully"
}
```

**Note:** Use the `reference_number` you provided (e.g., "SHP001") for the next steps!

---

### 2. Get Shipment Details

Retrieve shipment information by reference number:

```bash
grpcurl -plaintext -d '{
  "reference_number": "SHP001"
}' localhost:50052 shipment.ShipmentService/GetShipment
```

**Expected Response:**

```json
{
  "shipment": {
    "id": "uuid-here",
    "referenceNumber": "SHP001",
    "origin": "New York, NY",
    "destination": "Los Angeles, CA",
    "currentStatus": "STATUS_PENDING",
    "driver": {
      "driverId": "DRV-001",
      "driverName": "John Doe",
      "driverPhone": "+1-555-0123"
    },
    "unit": {
      "unitId": "VEH-001",
      "unitType": "Semi-Truck",
      "plateNumber": "NY-12345"
    },
    "shipmentAmount": 1250,
    "driverRevenue": 875,
    "createdAt": "2026-03-17T...",
    "updatedAt": "2026-03-17T..."
  }
}
```

---

### 3. Update Status: Picked Up

Mark the shipment as picked up from the warehouse:

```bash
grpcurl -plaintext -d '{
  "reference_number": "SHP001",
  "new_status": "STATUS_PICKED_UP",
  "location": "NYC Distribution Center",
  "description": "Shipment picked up by driver John Doe",
  "recorded_by": "system"
}' localhost:50052 shipment.ShipmentService/AddStatusEvent
```

**Expected Response:**

```json
{
  "event": {
    "id": "uuid-event",
    "shipmentId": "uuid-shipment",
    "status": "STATUS_PICKED_UP",
    "description": "Shipment picked up by driver John Doe",
    "location": "NYC Distribution Center",
    "occurredAt": "2026-03-17T...",
    "recordedBy": "system"
  },
  "updatedShipment": {
    "currentStatus": "STATUS_PICKED_UP",
    ...
  },
  "message": "Status event added successfully"
}
```

---

### 4. Update Status: In Transit

Update shipment to in-transit status:

```bash
grpcurl -plaintext -d '{
  "reference_number": "SHP001",
  "new_status": "STATUS_IN_TRANSIT",
  "location": "Pennsylvania Highway I-76",
  "description": "Shipment en route to destination",
  "recorded_by": "system"
}' localhost:50052 shipment.ShipmentService/AddStatusEvent
```

---

### 5. Update Status: Delivered

Mark shipment as successfully delivered (terminal state):

```bash
grpcurl -plaintext -d '{
  "reference_number": "SHP001",
  "new_status": "STATUS_DELIVERED",
  "location": "Los Angeles, CA - Customer Warehouse",
  "description": "Shipment delivered successfully. Signed by: Jane Smith",
  "recorded_by": "DRV-001"
}' localhost:50052 shipment.ShipmentService/AddStatusEvent
```

---

### 6. View Complete Shipment History

Get the full audit trail of all status changes:

```bash
grpcurl -plaintext -d '{
  "reference_number": "SHP001"
}' localhost:50052 shipment.ShipmentService/GetShipmentHistory
```

**Expected Response:**

```json
{
  "events": [
    {
      "id": "uuid-1",
      "shipmentId": "uuid-shipment",
      "status": "STATUS_PICKED_UP",
      "location": "NYC Distribution Center",
      "description": "Shipment picked up by driver John Doe",
      "occurredAt": "2026-03-17T10:00:00Z",
      "recordedBy": "system"
    },
    {
      "id": "uuid-2",
      "shipmentId": "uuid-shipment",
      "status": "STATUS_IN_TRANSIT",
      "location": "Pennsylvania Highway I-76",
      "description": "Shipment en route to destination",
      "occurredAt": "2026-03-17T14:30:00Z",
      "recordedBy": "system"
    },
    {
      "id": "uuid-3",
      "shipmentId": "uuid-shipment",
      "status": "STATUS_DELIVERED",
      "location": "Los Angeles, CA - Customer Warehouse",
      "description": "Shipment delivered successfully. Signed by: Jane Smith",
      "occurredAt": "2026-03-17T18:00:00Z",
      "recordedBy": "DRV-001"
    }
  ]
}
```

---

## Error Testing Scenarios

### Test Invalid Status Transition

Try to update a delivered shipment (should fail - terminal state):

```bash
grpcurl -plaintext -d '{
  "reference_number": "SHP001",
  "new_status": "STATUS_IN_TRANSIT",
  "location": "Somewhere",
  "description": "This should fail",
  "recorded_by": "system"
}' localhost:50052 shipment.ShipmentService/AddStatusEvent
```

**Expected Error:**

```json
{
  "error": "cannot modify shipment in terminal state"
}
```

---

### Test Invalid Transition Path

Try an invalid status jump (PENDING → DELIVERED without intermediate states):

Create a new shipment and try to deliver it immediately:

```bash
# Create shipment
grpcurl -plaintext -d '{
  "reference_number": "SHP002",
  "origin": "Seattle, WA",
  "destination": "Portland, OR",
  "driver": {
    "driver_id": "DRV-002",
    "driver_name": "Alice Smith",
    "driver_phone": "+1-555-0456"
  },
  "unit": {
    "unit_id": "VEH-002",
    "unit_type": "Van",
    "plate_number": "WA-67890"
  },
  "shipment_amount": 500.00,
  "driver_revenue": 350.00
}' localhost:50052 shipment.ShipmentService/CreateShipment

# Try to deliver immediately (should fail)
grpcurl -plaintext -d '{
  "reference_number": "SHP002",
  "new_status": "STATUS_DELIVERED",
  "location": "Portland, OR",
  "description": "This should fail - invalid transition",
  "recorded_by": "system"
}' localhost:50052 shipment.ShipmentService/AddStatusEvent
```

**Expected Error:**

```json
{
  "error": "invalid status transition from STATUS_PENDING to STATUS_DELIVERED"
}
```

---

### Test Non-Existent Shipment

Try to get a shipment that doesn't exist:

```bash
grpcurl -plaintext -d '{
  "reference_number": "SHP-NOTFOUND"
}' localhost:50052 shipment.ShipmentService/GetShipment
```

**Expected Error:**

```json
{
  "error": "shipment not found"
}
```

---
