package worker

import (
	"context"
	"log"
	"time"

	"vektor-shipment/services/shipment-service/internal/domain"
	"vektor-shipment/shared/messaging"
)

// OutboxWorker polls the outbox table and publishes pending events to RabbitMQ
type OutboxWorker struct {
	repo      domain.OutboxRepository
	publisher *messaging.RabbitMQPublisher
	interval  time.Duration
	batchSize int
	stopCh    chan struct{}
}

// NewOutboxWorker creates a new outbox worker
func NewOutboxWorker(
	repo domain.OutboxRepository,
	publisher *messaging.RabbitMQPublisher,
	interval time.Duration,
	batchSize int,
) *OutboxWorker {
	return &OutboxWorker{
		repo:      repo,
		publisher: publisher,
		interval:  interval,
		batchSize: batchSize,
		stopCh:    make(chan struct{}),
	}
}

// Start begins processing outbox events
func (w *OutboxWorker) Start(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	log.Printf("[OutboxWorker] Started with interval=%v, batchSize=%d", w.interval, w.batchSize)

	// Process immediately on start
	w.processOutbox(ctx)

	for {
		select {
		case <-ticker.C:
			w.processOutbox(ctx)
		case <-w.stopCh:
			log.Println("[OutboxWorker] Stopped")
			return
		case <-ctx.Done():
			log.Println("[OutboxWorker] Context cancelled, stopping...")
			return
		}
	}
}

// Stop stops the worker
func (w *OutboxWorker) Stop() {
	close(w.stopCh)
}

// processOutbox processes a batch of pending events
func (w *OutboxWorker) processOutbox(ctx context.Context) {
	events, err := w.repo.GetPendingEvents(ctx, w.batchSize)
	if err != nil {
		log.Printf("[OutboxWorker] Error fetching pending events: %v", err)
		return
	}

	if len(events) == 0 {
		return // No events to process
	}

	log.Printf("[OutboxWorker] Processing %d pending events", len(events))

	for _, event := range events {
		if err := w.publishEvent(ctx, event); err != nil {
			log.Printf("[OutboxWorker] Failed to publish event %s: %v", event.ID, err)
			w.handlePublishFailure(ctx, event, err)
		} else {
			log.Printf("[OutboxWorker] Successfully published event %s (type=%s, routing_key=%s)",
				event.ID, event.EventType, event.RoutingKey)
			if err := w.repo.MarkAsProcessed(ctx, event.ID); err != nil {
				log.Printf("[OutboxWorker] Failed to mark event %s as processed: %v", event.ID, err)
			}
		}
	}
}

// publishEvent publishes a single event to RabbitMQ
func (w *OutboxWorker) publishEvent(ctx context.Context, event *domain.OutboxEvent) error {
	return w.publisher.Publish(ctx, messaging.Message{
		RoutingKey:  event.RoutingKey,
		ContentType: "application/json",
		Body:        event.Payload,
	})
}

// handlePublishFailure handles failures during event publishing
func (w *OutboxWorker) handlePublishFailure(ctx context.Context, event *domain.OutboxEvent, publishErr error) {
	const maxAttempts = 5

	if event.Attempts+1 >= maxAttempts {
		// Max retries reached, mark as failed
		err := w.repo.MarkAsFailed(ctx, event.ID, publishErr.Error())
		if err != nil {
			log.Printf("[OutboxWorker] Failed to mark event %s as failed: %v", event.ID, err)
		} else {
			log.Printf("[OutboxWorker] Event %s marked as failed after %d attempts", event.ID, maxAttempts)
		}
	} else {
		// Increment attempt counter
		err := w.repo.IncrementAttempt(ctx, event.ID, publishErr.Error())
		if err != nil {
			log.Printf("[OutboxWorker] Failed to increment attempt for event %s: %v", event.ID, err)
		} else {
			log.Printf("[OutboxWorker] Event %s attempt incremented to %d", event.ID, event.Attempts+1)
		}
	}
}
