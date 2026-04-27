package service_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"urlshortener/internal/eventbus"
	"urlshortener/internal/service"
)

// --- моки ---

type mockGenerator struct {
	id  int64
	err error
}

func (m *mockGenerator) NextID() (int64, error) { return m.id, m.err }

type mockEncoder struct {
	code string
	err  error
}

func (m *mockEncoder) Encode(int64) (string, error) { return m.code, m.err }
func (m *mockEncoder) Decode(string) (int64, error) { return 0, nil }

// --- тесты ---

func TestGenerate_ReturnsCorrectNumericID(t *testing.T) {
	svc := service.New(&mockGenerator{id: 12345}, &mockEncoder{code: "3d7"}, eventbus.New())

	result, err := svc.Generate()

	require.NoError(t, err)
	assert.Equal(t, int64(12345), result.NumericID)
}

func TestGenerate_ReturnsCorrectShortCode(t *testing.T) {
	svc := service.New(&mockGenerator{id: 1}, &mockEncoder{code: "abc"}, eventbus.New())

	result, err := svc.Generate()

	require.NoError(t, err)
	assert.Equal(t, "abc", result.ShortCode)
}

func TestGenerate_GeneratorError_ReturnsError(t *testing.T) {
	svc := service.New(
		&mockGenerator{err: errors.New("clock moved backwards")},
		&mockEncoder{code: "x"},
		eventbus.New(),
	)

	_, err := svc.Generate()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to generate id")
}

func TestGenerate_EncoderError_ReturnsError(t *testing.T) {
	svc := service.New(
		&mockGenerator{id: 1},
		&mockEncoder{err: errors.New("overflow")},
		eventbus.New(),
	)

	_, err := svc.Generate()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to encode id")
}

func TestGenerate_PublishesIDGeneratedEvent(t *testing.T) {
	bus := eventbus.New()
	var received *eventbus.IDGeneratedEvent
	_ = bus.SubscribeIDGenerated(func(e eventbus.IDGeneratedEvent) {
		received = &e
	})

	svc := service.New(&mockGenerator{id: 999}, &mockEncoder{code: "fZ"}, bus)
	_, err := svc.Generate()

	require.NoError(t, err)
	require.NotNil(t, received, "IDGeneratedEvent must be published")
	assert.Equal(t, int64(999), received.NumericID)
	assert.Equal(t, "fZ", received.ShortCode)
	assert.False(t, received.Timestamp.IsZero())
}

func TestGenerate_GeneratorError_PublishesIDErrorEvent(t *testing.T) {
	bus := eventbus.New()
	var received *eventbus.IDErrorEvent
	_ = bus.SubscribeIDError(func(e eventbus.IDErrorEvent) {
		received = &e
	})

	svc := service.New(
		&mockGenerator{err: errors.New("boom")},
		&mockEncoder{},
		bus,
	)
	_, _ = svc.Generate()

	require.NotNil(t, received, "IDErrorEvent must be published on generator error")
	assert.False(t, received.Timestamp.IsZero())
}

func TestGenerate_EncoderError_PublishesIDErrorEvent(t *testing.T) {
	bus := eventbus.New()
	var received *eventbus.IDErrorEvent
	_ = bus.SubscribeIDError(func(e eventbus.IDErrorEvent) {
		received = &e
	})

	svc := service.New(
		&mockGenerator{id: 1},
		&mockEncoder{err: errors.New("encode failed")},
		bus,
	)
	_, _ = svc.Generate()

	require.NotNil(t, received, "IDErrorEvent must be published on encoder error")
}

func TestGenerate_NoErrorEvent_OnSuccess(t *testing.T) {
	bus := eventbus.New()
	var errEvent *eventbus.IDErrorEvent
	_ = bus.SubscribeIDError(func(e eventbus.IDErrorEvent) {
		errEvent = &e
	})

	svc := service.New(&mockGenerator{id: 1}, &mockEncoder{code: "1"}, bus)
	_, err := svc.Generate()

	require.NoError(t, err)
	assert.Nil(t, errEvent, "no IDErrorEvent must be published on success")
}
