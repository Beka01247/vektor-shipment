package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"vektor-shipment/services/shipment-service/internal/domain"

	_ "github.com/lib/pq"
)

// PostgresRepository implements the ShipmentRepository interface using PostgreSQL
type PostgresRepository struct {
	db *sql.DB
}

// NewPostgresRepository creates a new PostgreSQL repository
func NewPostgresRepository(db *sql.DB) *PostgresRepository {
	return &PostgresRepository{db: db}
}

// Create creates a new shipment in the database
func (r *PostgresRepository) Create(ctx context.Context, shipment *domain.Shipment) error {
	query := `
		INSERT INTO shipments (
			id, reference_number, origin, destination, current_status,
			driver_id, driver_name, driver_phone,
			unit_id, unit_type, plate_number,
			shipment_amount, driver_revenue, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
	`

	_, err := r.db.ExecContext(ctx, query,
		shipment.ID,
		shipment.ReferenceNumber,
		shipment.Origin,
		shipment.Destination,
		int32(shipment.CurrentStatus),
		shipment.Driver.DriverID,
		shipment.Driver.DriverName,
		shipment.Driver.DriverPhone,
		shipment.Unit.UnitID,
		shipment.Unit.UnitType,
		shipment.Unit.PlateNumber,
		shipment.ShipmentAmount,
		shipment.DriverRevenue,
		shipment.CreatedAt,
		shipment.UpdatedAt,
	)

	if err != nil {
		// Check for unique constraint violation
		if isDuplicateKeyError(err) {
			return domain.ErrDuplicateReference
		}
		return fmt.Errorf("failed to insert shipment: %w", err)
	}

	return nil
}

// GetByReferenceNumber retrieves a shipment by its reference number
func (r *PostgresRepository) GetByReferenceNumber(ctx context.Context, refNumber string) (*domain.Shipment, error) {
	query := `
		SELECT 
			id, reference_number, origin, destination, current_status,
			driver_id, driver_name, driver_phone,
			unit_id, unit_type, plate_number,
			shipment_amount, driver_revenue, created_at, updated_at
		FROM shipments
		WHERE reference_number = $1
	`

	var shipment domain.Shipment
	var status int32

	err := r.db.QueryRowContext(ctx, query, refNumber).Scan(
		&shipment.ID,
		&shipment.ReferenceNumber,
		&shipment.Origin,
		&shipment.Destination,
		&status,
		&shipment.Driver.DriverID,
		&shipment.Driver.DriverName,
		&shipment.Driver.DriverPhone,
		&shipment.Unit.UnitID,
		&shipment.Unit.UnitType,
		&shipment.Unit.PlateNumber,
		&shipment.ShipmentAmount,
		&shipment.DriverRevenue,
		&shipment.CreatedAt,
		&shipment.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrShipmentNotFound
		}
		return nil, fmt.Errorf("failed to query shipment: %w", err)
	}

	shipment.CurrentStatus = domain.ShipmentStatus(status)
	return &shipment, nil
}

// Update updates an existing shipment
func (r *PostgresRepository) Update(ctx context.Context, shipment *domain.Shipment) error {
	query := `
		UPDATE shipments
		SET current_status = $1, updated_at = $2
		WHERE id = $3
	`

	result, err := r.db.ExecContext(ctx, query,
		int32(shipment.CurrentStatus),
		shipment.UpdatedAt,
		shipment.ID,
	)

	if err != nil {
		return fmt.Errorf("failed to update shipment: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return domain.ErrShipmentNotFound
	}

	return nil
}

// AddStatusEvent adds a new status change event
func (r *PostgresRepository) AddStatusEvent(ctx context.Context, event *domain.StatusEvent) error {
	query := `
		INSERT INTO status_events (
			id, shipment_id, status, description, location, occurred_at, recorded_by
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
	`

	_, err := r.db.ExecContext(ctx, query,
		event.ID,
		event.ShipmentID,
		int32(event.Status),
		event.Description,
		event.Location,
		event.OccurredAt,
		event.RecordedBy,
	)

	if err != nil {
		return fmt.Errorf("failed to insert status event: %w", err)
	}

	return nil
}

// GetStatusEvents retrieves all status events for a shipment, ordered by occurrence time
func (r *PostgresRepository) GetStatusEvents(ctx context.Context, shipmentID string) ([]*domain.StatusEvent, error) {
	query := `
		SELECT id, shipment_id, status, description, location, occurred_at, recorded_by
		FROM status_events
		WHERE shipment_id = $1
		ORDER BY occurred_at ASC
	`

	rows, err := r.db.QueryContext(ctx, query, shipmentID)
	if err != nil {
		return nil, fmt.Errorf("failed to query status events: %w", err)
	}
	defer rows.Close()

	var events []*domain.StatusEvent

	for rows.Next() {
		var event domain.StatusEvent
		var status int32

		err := rows.Scan(
			&event.ID,
			&event.ShipmentID,
			&status,
			&event.Description,
			&event.Location,
			&event.OccurredAt,
			&event.RecordedBy,
		)

		if err != nil {
			return nil, fmt.Errorf("failed to scan status event: %w", err)
		}

		event.Status = domain.ShipmentStatus(status)
		events = append(events, &event)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating status events: %w", err)
	}

	return events, nil
}

// isDuplicateKeyError checks if the error is a duplicate key constraint violation
func isDuplicateKeyError(err error) bool {
	if err == nil {
		return false
	}
	// PostgreSQL error code 23505 is unique_violation
	return errors.Is(err, sql.ErrNoRows) == false &&
		(fmt.Sprintf("%v", err) == "pq: duplicate key value violates unique constraint \"shipments_reference_number_key\"" ||
			fmt.Sprintf("%v", err) == "UNIQUE constraint failed: shipments.reference_number")
}

// InitSchema initializes the database schema
func (r *PostgresRepository) InitSchema(ctx context.Context) error {
	schema := `
		CREATE TABLE IF NOT EXISTS shipments (
			id VARCHAR(36) PRIMARY KEY,
			reference_number VARCHAR(100) UNIQUE NOT NULL,
			origin TEXT NOT NULL,
			destination TEXT NOT NULL,
			current_status INTEGER NOT NULL,
			driver_id VARCHAR(36) NOT NULL,
			driver_name VARCHAR(255) NOT NULL,
			driver_phone VARCHAR(50) NOT NULL,
			unit_id VARCHAR(36) NOT NULL,
			unit_type VARCHAR(100) NOT NULL,
			plate_number VARCHAR(50) NOT NULL,
			shipment_amount DECIMAL(10, 2) NOT NULL,
			driver_revenue DECIMAL(10, 2) NOT NULL,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		);

		CREATE INDEX IF NOT EXISTS idx_shipments_reference ON shipments(reference_number);
		CREATE INDEX IF NOT EXISTS idx_shipments_status ON shipments(current_status);

		CREATE TABLE IF NOT EXISTS status_events (
			id VARCHAR(36) PRIMARY KEY,
			shipment_id VARCHAR(36) NOT NULL,
			status INTEGER NOT NULL,
			description TEXT,
			location TEXT,
			occurred_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			recorded_by VARCHAR(255) NOT NULL,
			FOREIGN KEY (shipment_id) REFERENCES shipments(id) ON DELETE CASCADE
		);

		CREATE INDEX IF NOT EXISTS idx_status_events_shipment ON status_events(shipment_id);
		CREATE INDEX IF NOT EXISTS idx_status_events_occurred ON status_events(occurred_at);

		-- Transactional Outbox Table
		CREATE TABLE IF NOT EXISTS outbox_events (
			id VARCHAR(36) PRIMARY KEY,
			aggregate_id VARCHAR(36) NOT NULL,
			event_type VARCHAR(100) NOT NULL,
			payload JSONB NOT NULL,
			routing_key VARCHAR(255) NOT NULL,
			status VARCHAR(20) NOT NULL DEFAULT 'pending',
			attempts INTEGER NOT NULL DEFAULT 0,
			last_attempt TIMESTAMP,
			error_message TEXT,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			processed_at TIMESTAMP
		);

		CREATE INDEX IF NOT EXISTS idx_outbox_status ON outbox_events(status);
		CREATE INDEX IF NOT EXISTS idx_outbox_created ON outbox_events(created_at);
		CREATE INDEX IF NOT EXISTS idx_outbox_aggregate ON outbox_events(aggregate_id);
	`

	_, err := r.db.ExecContext(ctx, schema)
	if err != nil {
		return fmt.Errorf("failed to initialize schema: %w", err)
	}

	return nil
}

// ============================================================================
// Transactional Outbox Pattern Implementation
// ============================================================================

// SaveOutboxEvent saves an event to the outbox table
func (r *PostgresRepository) SaveOutboxEvent(ctx context.Context, event *domain.OutboxEvent) error {
	query := `
		INSERT INTO outbox_events (
			id, aggregate_id, event_type, payload, routing_key, status, 
			attempts, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`

	_, err := r.db.ExecContext(ctx, query,
		event.ID,
		event.AggregateID,
		event.EventType,
		event.Payload,
		event.RoutingKey,
		event.Status,
		event.Attempts,
		event.CreatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to save outbox event: %w", err)
	}

	return nil
}

// GetPendingEvents retrieves pending events for processing
func (r *PostgresRepository) GetPendingEvents(ctx context.Context, limit int) ([]*domain.OutboxEvent, error) {
	query := `
		SELECT 
			id, aggregate_id, event_type, payload, routing_key, status,
			attempts, last_attempt, error_message, created_at, processed_at
		FROM outbox_events
		WHERE status = 'pending' AND attempts < 5
		ORDER BY created_at ASC
		LIMIT $1
	`

	rows, err := r.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query pending events: %w", err)
	}
	defer rows.Close()

	var events []*domain.OutboxEvent

	for rows.Next() {
		var event domain.OutboxEvent
		var lastAttempt sql.NullTime
		var processedAt sql.NullTime
		var errorMsg sql.NullString

		err := rows.Scan(
			&event.ID,
			&event.AggregateID,
			&event.EventType,
			&event.Payload,
			&event.RoutingKey,
			&event.Status,
			&event.Attempts,
			&lastAttempt,
			&errorMsg,
			&event.CreatedAt,
			&processedAt,
		)

		if err != nil {
			return nil, fmt.Errorf("failed to scan outbox event: %w", err)
		}

		if lastAttempt.Valid {
			event.LastAttempt = &lastAttempt.Time
		}
		if processedAt.Valid {
			event.ProcessedAt = &processedAt.Time
		}
		if errorMsg.Valid {
			event.ErrorMessage = errorMsg.String
		}

		events = append(events, &event)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating outbox events: %w", err)
	}

	return events, nil
}

// MarkAsProcessed marks an outbox event as successfully processed
func (r *PostgresRepository) MarkAsProcessed(ctx context.Context, eventID string) error {
	query := `
		UPDATE outbox_events
		SET status = 'processed', processed_at = $1
		WHERE id = $2
	`

	_, err := r.db.ExecContext(ctx, query, sql.NullTime{Time: sql.NullTime{}.Time, Valid: true}, eventID)
	if err != nil {
		return fmt.Errorf("failed to mark event as processed: %w", err)
	}

	return nil
}

// MarkAsFailed marks an outbox event as failed after max retries
func (r *PostgresRepository) MarkAsFailed(ctx context.Context, eventID string, errorMsg string) error {
	query := `
		UPDATE outbox_events
		SET status = 'failed', error_message = $1, last_attempt = $2
		WHERE id = $3
	`

	_, err := r.db.ExecContext(ctx, query, errorMsg, sql.NullTime{}.Time, eventID)
	if err != nil {
		return fmt.Errorf("failed to mark event as failed: %w", err)
	}

	return nil
}

// IncrementAttempt increments the attempt counter for an event
func (r *PostgresRepository) IncrementAttempt(ctx context.Context, eventID string, errorMsg string) error {
	query := `
		UPDATE outbox_events
		SET attempts = attempts + 1, last_attempt = $1, error_message = $2
		WHERE id = $3
	`

	_, err := r.db.ExecContext(ctx, query, sql.NullTime{}.Time, errorMsg, eventID)
	if err != nil {
		return fmt.Errorf("failed to increment attempt: %w", err)
	}

	return nil
}

// BeginTx begins a new transaction
func (r *PostgresRepository) BeginTx(ctx context.Context) (*sql.Tx, error) {
	return r.db.BeginTx(ctx, nil)
}

// CreateWithOutbox creates a shipment and outbox event in a single transaction
func (r *PostgresRepository) CreateWithOutbox(ctx context.Context, shipment *domain.Shipment, outboxEvent *domain.OutboxEvent) error {
	tx, err := r.BeginTx(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Insert shipment
	shipmentQuery := `
		INSERT INTO shipments (
			id, reference_number, origin, destination, current_status,
			driver_id, driver_name, driver_phone,
			unit_id, unit_type, plate_number,
			shipment_amount, driver_revenue, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
	`

	_, err = tx.ExecContext(ctx, shipmentQuery,
		shipment.ID,
		shipment.ReferenceNumber,
		shipment.Origin,
		shipment.Destination,
		int32(shipment.CurrentStatus),
		shipment.Driver.DriverID,
		shipment.Driver.DriverName,
		shipment.Driver.DriverPhone,
		shipment.Unit.UnitID,
		shipment.Unit.UnitType,
		shipment.Unit.PlateNumber,
		shipment.ShipmentAmount,
		shipment.DriverRevenue,
		shipment.CreatedAt,
		shipment.UpdatedAt,
	)

	if err != nil {
		if isDuplicateKeyError(err) {
			return domain.ErrDuplicateReference
		}
		return fmt.Errorf("failed to insert shipment: %w", err)
	}

	// Insert outbox event
	outboxQuery := `
		INSERT INTO outbox_events (
			id, aggregate_id, event_type, payload, routing_key, status, 
			attempts, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`

	_, err = tx.ExecContext(ctx, outboxQuery,
		outboxEvent.ID,
		outboxEvent.AggregateID,
		outboxEvent.EventType,
		outboxEvent.Payload,
		outboxEvent.RoutingKey,
		outboxEvent.Status,
		outboxEvent.Attempts,
		outboxEvent.CreatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to insert outbox event: %w", err)
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// UpdateWithOutbox updates a shipment and creates an outbox event in a single transaction
func (r *PostgresRepository) UpdateWithOutbox(ctx context.Context, shipment *domain.Shipment, statusEvent *domain.StatusEvent, outboxEvent *domain.OutboxEvent) error {
	tx, err := r.BeginTx(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Update shipment
	shipmentQuery := `
		UPDATE shipments
		SET current_status = $1, updated_at = $2
		WHERE id = $3
	`

	result, err := tx.ExecContext(ctx, shipmentQuery,
		int32(shipment.CurrentStatus),
		shipment.UpdatedAt,
		shipment.ID,
	)

	if err != nil {
		return fmt.Errorf("failed to update shipment: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return domain.ErrShipmentNotFound
	}

	// Insert status event
	statusQuery := `
		INSERT INTO status_events (
			id, shipment_id, status, description, location, occurred_at, recorded_by
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
	`

	_, err = tx.ExecContext(ctx, statusQuery,
		statusEvent.ID,
		statusEvent.ShipmentID,
		int32(statusEvent.Status),
		statusEvent.Description,
		statusEvent.Location,
		statusEvent.OccurredAt,
		statusEvent.RecordedBy,
	)

	if err != nil {
		return fmt.Errorf("failed to insert status event: %w", err)
	}

	// Insert outbox event
	outboxQuery := `
		INSERT INTO outbox_events (
			id, aggregate_id, event_type, payload, routing_key, status, 
			attempts, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`

	_, err = tx.ExecContext(ctx, outboxQuery,
		outboxEvent.ID,
		outboxEvent.AggregateID,
		outboxEvent.EventType,
		outboxEvent.Payload,
		outboxEvent.RoutingKey,
		outboxEvent.Status,
		outboxEvent.Attempts,
		outboxEvent.CreatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to insert outbox event: %w", err)
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}
