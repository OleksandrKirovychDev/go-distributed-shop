// Package events encodes domain facts into the proto OutboxEvent rows the
// publisher persists. It owns the generated event types and proto marshalling
// so the app layer stays free of protobuf.
package events

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	identityeventsv1 "github.com/online-shop/proto/gen/go/events/identity/v1"

	"github.com/online-shop/services/identity/internal/app/ports"
	"github.com/online-shop/services/identity/internal/domain"
)

const (
	topicUserRegistered = "identity.user.registered.v1"
	aggregateTypeUser   = "user"
)

type Encoder struct{}

func NewEncoder() *Encoder { return &Encoder{} }

func (e *Encoder) UserRegistered(u *domain.User, now time.Time) (ports.OutboxEvent, error) {
	roles := make([]string, len(u.Roles))
	for i, r := range u.Roles {
		roles[i] = r.String()
	}

	payload := &identityeventsv1.UserRegistered{
		EventId:       uuid.NewString(),
		OccurredAt:    timestamppb.New(now),
		AggregateId:   u.ID.String(),
		AggregateType: aggregateTypeUser,
		Version:       1,
		UserId:        u.ID.String(),
		Email:         u.Email.String(),
		Roles:         roles,
	}

	marshalled, err := proto.Marshal(payload)
	if err != nil {
		return ports.OutboxEvent{}, fmt.Errorf("encode user registered: %w", err)
	}

	return ports.OutboxEvent{
		AggregateID: u.ID.String(),
		Topic:       topicUserRegistered,
		Key:         []byte(u.ID.String()),
		Payload:     marshalled,
	}, nil
}

var _ ports.EventEncoder = (*Encoder)(nil)
