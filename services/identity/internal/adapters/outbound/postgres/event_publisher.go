package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"

	"github.com/online-shop/pkg/errors"
	pkgpg "github.com/online-shop/pkg/postgres"

	"github.com/online-shop/services/identity/internal/adapters/outbound/postgres/gen"
	"github.com/online-shop/services/identity/internal/app/ports"
)

// EventPublisher writes outbox rows. Built over a Querier, so the TxManager can
// bind it to the same pgx.Tx as the aggregate write — that shared tx is the
// whole point of the transactional outbox.
type EventPublisher struct {
	q *gen.Queries
}

func NewEventPublisher(db pkgpg.Querier) *EventPublisher {
	return &EventPublisher{q: gen.New(db)}
}

func (p *EventPublisher) Publish(ctx context.Context, event ports.OutboxEvent) error {
	aggregateID, err := uuid.Parse(event.AggregateID)
	if err != nil {
		return errors.NewInternal("identity.bad_aggregate_id", "aggregate id is not a uuid", err)
	}

	headers := []byte("{}")
	if len(event.Headers) > 0 {
		headers, err = json.Marshal(event.Headers)
		if err != nil {
			return fmt.Errorf("marshal outbox headers: %w", err)
		}
	}

	if err := p.q.InsertOutboxEvent(ctx, gen.InsertOutboxEventParams{
		AggregateID: aggregateID,
		Topic:       event.Topic,
		Key:         event.Key,
		Payload:     event.Payload,
		Headers:     headers,
	}); err != nil {
		return fmt.Errorf("insert outbox event: %w", err)
	}
	return nil
}

var _ ports.EventPublisher = (*EventPublisher)(nil)
