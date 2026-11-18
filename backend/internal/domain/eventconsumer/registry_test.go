package eventconsumer

import (
	"testing"

	"laguna-escondida/backend/internal/domain/ports/mocks"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func consumer(t *testing.T, name string, events ...string) *mocks.MockEventConsumer {
	c := mocks.NewMockEventConsumer(t)
	c.EXPECT().Name().Return(name).Maybe()
	c.EXPECT().Handles().Return(events).Maybe()
	return c
}

func TestRegistry_DuplicateName_Errors(t *testing.T) {
	_, err := NewRegistry(consumer(t, "a", "x"), consumer(t, "a", "y"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), `"a"`)
}

func TestRegistry_EmptyName_Errors(t *testing.T) {
	_, err := NewRegistry(consumer(t, "", "x"))
	require.Error(t, err)
}

func TestRegistry_LookupByNameAndEvent(t *testing.T) {
	a, b := consumer(t, "a", "x", "y"), consumer(t, "b", "x")
	r, err := NewRegistry(a, b)
	require.NoError(t, err)

	assert.Equal(t, []string{"a", "b"}, r.ConsumersFor("x"))
	assert.Equal(t, []string{"a"}, r.ConsumersFor("y"))
	assert.Empty(t, r.ConsumersFor("z"))
	got, ok := r.Get("b")
	require.True(t, ok)
	assert.Same(t, b, got)
	_, ok = r.Get("missing")
	assert.False(t, ok)
}
