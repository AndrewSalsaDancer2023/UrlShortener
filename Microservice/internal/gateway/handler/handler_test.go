package handler_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	//	client "urlshortener/internal/gateway/client/http"
	"urlshortener/internal/gateway/handler"
	client "urlshortener/internal/gateway/handler/domain"
)

// --- мок клиента ---

type mockClient struct {
	resp *client.GenerateResponse
	err  error
}

func (m *mockClient) Generate(_ context.Context) (*client.GenerateResponse, error) {
	return m.resp, m.err
}

// --- фабрики ---

func newRouter(t *testing.T, c *mockClient) http.Handler {
	t.Helper()
	return handler.New(c).NewRouter()
}

func do(router http.Handler, method, path string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

// =============================================================================
// POST /api/v1/shorten
// =============================================================================

func TestShorten_StatusOK(t *testing.T) {
	router := newRouter(t, &mockClient{
		resp: &client.GenerateResponse{NumericID: 123, ShortCode: "abc"},
	})
	rec := do(router, http.MethodPost, "/api/v1/shorten")
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestShorten_ContentTypeJSON(t *testing.T) {
	router := newRouter(t, &mockClient{
		resp: &client.GenerateResponse{NumericID: 1, ShortCode: "x"},
	})
	rec := do(router, http.MethodPost, "/api/v1/shorten")
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
}

func TestShorten_BodyContainsNumericID(t *testing.T) {
	router := newRouter(t, &mockClient{
		resp: &client.GenerateResponse{NumericID: 42, ShortCode: "G"},
	})
	rec := do(router, http.MethodPost, "/api/v1/shorten")

	var body map[string]any
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
	assert.Contains(t, body, "numeric_id")
}

func TestShorten_BodyContainsShortCode(t *testing.T) {
	router := newRouter(t, &mockClient{
		resp: &client.GenerateResponse{NumericID: 1, ShortCode: "fZ"},
	})
	rec := do(router, http.MethodPost, "/api/v1/shorten")

	var body map[string]any
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
	assert.Equal(t, "fZ", body["short_code"])
}

func TestShorten_BodyContainsShortURL(t *testing.T) {
	router := newRouter(t, &mockClient{
		resp: &client.GenerateResponse{NumericID: 1, ShortCode: "fZ"},
	})
	rec := do(router, http.MethodPost, "/api/v1/shorten")

	var body map[string]any
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
	assert.Contains(t, body["short_url"].(string), "fZ")
}

func TestShorten_UpstreamError_Returns502(t *testing.T) {
	router := newRouter(t, &mockClient{err: errors.New("connection refused")})
	rec := do(router, http.MethodPost, "/api/v1/shorten")
	assert.Equal(t, http.StatusBadGateway, rec.Code)
}

func TestShorten_UpstreamError_BodyContainsError(t *testing.T) {
	router := newRouter(t, &mockClient{err: errors.New("timeout")})
	rec := do(router, http.MethodPost, "/api/v1/shorten")

	var body map[string]any
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
	assert.Contains(t, body, "error")
}

func TestShorten_WrongMethod_GET_Returns405(t *testing.T) {
	router := newRouter(t, &mockClient{})
	rec := do(router, http.MethodGet, "/api/v1/shorten")
	assert.Equal(t, http.StatusMethodNotAllowed, rec.Code)
}

func TestShorten_WrongMethod_PUT_Returns405(t *testing.T) {
	router := newRouter(t, &mockClient{})
	rec := do(router, http.MethodPut, "/api/v1/shorten")
	assert.Equal(t, http.StatusMethodNotAllowed, rec.Code)
}

// =============================================================================
// GET /health
// =============================================================================

func TestHealth_StatusOK(t *testing.T) {
	router := newRouter(t, &mockClient{})
	rec := do(router, http.MethodGet, "/health")
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestHealth_BodyContainsStatusOK(t *testing.T) {
	router := newRouter(t, &mockClient{})
	rec := do(router, http.MethodGet, "/health")

	var body map[string]string
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
	assert.Equal(t, "ok", body["status"])
}

func TestHealth_WrongMethod_Returns405(t *testing.T) {
	router := newRouter(t, &mockClient{})
	rec := do(router, http.MethodPost, "/health")
	assert.Equal(t, http.StatusMethodNotAllowed, rec.Code)
}

// =============================================================================
// 404
// =============================================================================

func TestUnknownRoute_Returns404(t *testing.T) {
	router := newRouter(t, &mockClient{})
	rec := do(router, http.MethodGet, "/api/v1/nonexistent")
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestUnknownRoute_BodyContainsError(t *testing.T) {
	router := newRouter(t, &mockClient{})
	rec := do(router, http.MethodGet, "/nonexistent")

	var body map[string]any
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
	assert.Contains(t, body, "error")
}
