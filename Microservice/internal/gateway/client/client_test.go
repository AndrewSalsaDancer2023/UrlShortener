package client_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	client "urlshortener/internal/gateway/client/http"
)

// newFakeUpstream поднимает тестовый HTTP-сервер, имитирующий микросервис генерации ID.
func newFakeUpstream(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return srv
}

// =============================================================================
// Generate
// =============================================================================

func TestGenerate_Success(t *testing.T) {
	srv := newFakeUpstream(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"numeric_id": 12345,
			"short_code": "3d7",
		})
	})

	c, _ := client.New(srv.URL, 5*time.Second)
	resp, err := c.Generate(context.Background())

	require.NoError(t, err)
	assert.Equal(t, int64(12345), resp.NumericID)
	assert.Equal(t, "3d7", resp.ShortCode)
}

func TestGenerate_UsesPostMethod(t *testing.T) {
	var receivedMethod string
	srv := newFakeUpstream(t, func(w http.ResponseWriter, r *http.Request) {
		receivedMethod = r.Method
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"numeric_id": 1, "short_code": "1"})
	})

	c, _ := client.New(srv.URL, 5*time.Second)
	_, err := c.Generate(context.Background())

	require.NoError(t, err)
	assert.Equal(t, http.MethodPost, receivedMethod)
}

func TestGenerate_HitsCorrectPath(t *testing.T) {
	var receivedPath string
	srv := newFakeUpstream(t, func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"numeric_id": 1, "short_code": "1"})
	})

	c, _ := client.New(srv.URL, 5*time.Second)
	_, err := c.Generate(context.Background())

	require.NoError(t, err)
	assert.Equal(t, "/api/v1/generate", receivedPath)
}

func TestGenerate_Upstream500_ReturnsError(t *testing.T) {
	srv := newFakeUpstream(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	c, _ := client.New(srv.URL, 5*time.Second)
	_, err := c.Generate(context.Background())

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "500")
}

func TestGenerate_Upstream404_ReturnsError(t *testing.T) {
	srv := newFakeUpstream(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	c, _ := client.New(srv.URL, 5*time.Second)
	_, err := c.Generate(context.Background())

	assert.Error(t, err)
}

func TestGenerate_InvalidJSON_ReturnsError(t *testing.T) {
	srv := newFakeUpstream(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("not json"))
	})

	c, _ := client.New(srv.URL, 5*time.Second)
	_, err := c.Generate(context.Background())

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "decode response")
}

func TestGenerate_Timeout_ReturnsError(t *testing.T) {
	srv := newFakeUpstream(t, func(w http.ResponseWriter, r *http.Request) {
		// Имитируем зависший upstream — спим дольше таймаута клиента
		time.Sleep(200 * time.Millisecond)
	})

	c, _ := client.New(srv.URL, 50*time.Millisecond)
	_, err := c.Generate(context.Background())

	assert.Error(t, err, "must return error on timeout")
}

func TestGenerate_ContextCancelled_ReturnsError(t *testing.T) {
	srv := newFakeUpstream(t, func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // отменяем контекст сразу

	c, _ := client.New(srv.URL, 5*time.Second)
	_, err := c.Generate(ctx)

	assert.Error(t, err)
}

func TestGenerate_UnreachableHost_ReturnsError(t *testing.T) {
	c, _ := client.New("http://127.0.0.1:1", 100*time.Millisecond)
	_, err := c.Generate(context.Background())
	assert.Error(t, err)
}
