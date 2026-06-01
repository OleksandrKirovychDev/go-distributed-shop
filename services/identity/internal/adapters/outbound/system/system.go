// Package system supplies the ambient outbound ports — wall-clock time and ID
// generation — as thin adapters so use cases stay deterministic under test
// (fixed clock / fixed IDs) while production uses real time and UUIDs.
package system

import (
	"time"

	"github.com/google/uuid"
)

type Clock struct{}

func NewClock() Clock { return Clock{} }

func (Clock) Now() time.Time { return time.Now() }

type IDGenerator struct{}

func NewIDGenerator() IDGenerator { return IDGenerator{} }

func (IDGenerator) NewID() string { return uuid.NewString() }
