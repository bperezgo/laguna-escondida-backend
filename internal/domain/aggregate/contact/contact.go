package contact

import (
	"strings"
	"time"

	"laguna-escondida/backend/internal/domain/aggregate/shared"
	"laguna-escondida/backend/internal/domain/dto"

	"github.com/google/uuid"
)

const maxNameLength = 100

// Aggregate is one guest phone within an organization. Its first touch is written once.
type Aggregate struct {
	id          string
	phone       string
	name        *string
	firstTouch  *dto.Attribution
	firstSeenAt time.Time
	lastSeenAt  time.Time
	createdAt   time.Time
	updatedAt   time.Time
}

func New(touch dto.ContactTouch) *Aggregate {
	a := &Aggregate{
		id:          uuid.Must(uuid.NewV7()).String(),
		phone:       touch.Phone,
		firstSeenAt: touch.At,
		createdAt:   touch.At,
	}
	a.Touch(touch)
	return a
}

func FromDTO(c *dto.Contact) *Aggregate {
	return &Aggregate{
		id:          c.ID,
		phone:       c.Phone,
		name:        c.Name,
		firstTouch:  c.FirstTouch,
		firstSeenAt: c.FirstSeenAt,
		lastSeenAt:  c.LastSeenAt,
		createdAt:   c.CreatedAt,
		updatedAt:   c.UpdatedAt,
	}
}

// Touch records a later reservation or agent order: the latest non-empty name wins, and the
// first attribution ever seen becomes the first touch for good.
func (a *Aggregate) Touch(touch dto.ContactTouch) {
	if name := cleanName(touch.Name); name != nil {
		a.name = name
	}
	if a.firstTouch == nil && !shared.AttributionIsEmpty(touch.Attribution) {
		firstTouch := *touch.Attribution
		a.firstTouch = &firstTouch
	}
	if touch.At.After(a.lastSeenAt) {
		a.lastSeenAt = touch.At
	}
	a.updatedAt = touch.At
}

func (a *Aggregate) ToDTO() *dto.Contact {
	return &dto.Contact{
		ID:          a.id,
		Phone:       a.phone,
		Name:        a.name,
		FirstTouch:  a.firstTouch,
		FirstSeenAt: a.firstSeenAt,
		LastSeenAt:  a.lastSeenAt,
		CreatedAt:   a.createdAt,
		UpdatedAt:   a.updatedAt,
	}
}

func cleanName(raw *string) *string {
	if raw == nil {
		return nil
	}
	name := strings.TrimSpace(*raw)
	if name == "" {
		return nil
	}
	if runes := []rune(name); len(runes) > maxNameLength {
		name = string(runes[:maxNameLength])
	}
	return &name
}
