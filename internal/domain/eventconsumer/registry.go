package eventconsumer

import (
	"fmt"

	"laguna-escondida/backend/internal/domain/ports"
)

// Registry holds the consumers by name; names are stored in deliveries, so they must be unique and stable.
type Registry struct {
	byName  map[string]ports.EventConsumer
	byEvent map[string][]string
}

func NewRegistry(consumers ...ports.EventConsumer) (*Registry, error) {
	r := &Registry{byName: map[string]ports.EventConsumer{}, byEvent: map[string][]string{}}
	for _, c := range consumers {
		name := c.Name()
		if name == "" {
			return nil, fmt.Errorf("event consumer has an empty name")
		}
		if _, dup := r.byName[name]; dup {
			return nil, fmt.Errorf("event consumer %q registered twice", name)
		}
		r.byName[name] = c
		for _, event := range c.Handles() {
			r.byEvent[event] = append(r.byEvent[event], name)
		}
	}
	return r, nil
}

func (r *Registry) Get(name string) (ports.EventConsumer, bool) {
	c, ok := r.byName[name]
	return c, ok
}

// ConsumersFor is the names of the consumers that handle an event, in registration order.
func (r *Registry) ConsumersFor(event string) []string {
	return append([]string(nil), r.byEvent[event]...)
}
