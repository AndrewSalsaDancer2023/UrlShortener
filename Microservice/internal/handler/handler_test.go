package handler_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"urlshortener/internal/base62"
	"urlshortener/internal/dbstorage/pool"
	"urlshortener/internal/generator"
	"urlshortener/internal/handler"
	service "urlshortener/internal/idgenservice"
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

// --- фабрики ---

// newRouter собирает роутер с реальными зависимостями.
func newRouter(t *testing.T) http.Handler {
	t.Helper()
	timeEngine := pool.UnixTimeReal{}
	gen, err := generator.New(&generator.Config{DatacenterID: 1, MachineID: 1}, timeEngine)
	require.NoError(t, err)
	svc := service.New(gen, base62.NewEncoder())
	h := handler.New(svc)
	return h.NewRouter()
}

// newRouterWithMocks собирает роутер с подменёнными зависимостями.
func newRouterWithMocks(t *testing.T, gen *mockGenerator, enc *mockEncoder) http.Handler {
	t.Helper()
	svc := service.New(gen, enc)
	h := handler.New(svc)
	return h.NewRouter()
}

// do выполняет запрос и возвращает ResponseRecorder.
func do(router http.Handler, method, path string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

// --- тесты POST /api/v1/generate ---

func TestGenerate_StatusOK(t *testing.T) {
	rec := do(newRouter(t), http.MethodPost, "/api/v1/generate")
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestGenerate_ContentTypeJSON(t *testing.T) {
	rec := do(newRouter(t), http.MethodPost, "/api/v1/generate")
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
}

func TestGenerate_BodyContainsNumericID(t *testing.T) {
	rec := do(newRouter(t), http.MethodPost, "/api/v1/generate")

	var body map[string]any
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
	assert.Contains(t, body, "numeric_id", "response must contain numeric_id")
}

func TestGenerate_BodyContainsShortCode(t *testing.T) {
	rec := do(newRouter(t), http.MethodPost, "/api/v1/generate")

	var body map[string]any
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
	assert.Contains(t, body, "short_code", "response must contain short_code")
}

func TestGenerate_ShortCodeNotEmpty(t *testing.T) {
	rec := do(newRouter(t), http.MethodPost, "/api/v1/generate")

	var body map[string]any
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
	assert.NotEmpty(t, body["short_code"])
}

func TestGenerate_NumericIDPositive(t *testing.T) {
	rec := do(newRouter(t), http.MethodPost, "/api/v1/generate")

	var body map[string]any
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
	// JSON числа декодируются в float64
	id, ok := body["numeric_id"].(float64)
	require.True(t, ok, "numeric_id must be a number")
	assert.Greater(t, id, float64(0))
}

func TestGenerate_IDsAreUnique(t *testing.T) {
	router := newRouter(t)
	seen := make(map[float64]struct{}, 100)

	for i := 0; i < 100; i++ {
		rec := do(router, http.MethodPost, "/api/v1/generate")
		require.Equal(t, http.StatusOK, rec.Code)

		var body map[string]any
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))

		id := body["numeric_id"].(float64)
		_, dup := seen[id]
		assert.False(t, dup, "duplicate ID detected: %v", id)
		seen[id] = struct{}{}
	}
}

func TestGenerate_GeneratorError_Returns500(t *testing.T) {
	router := newRouterWithMocks(t,
		&mockGenerator{err: errors.New("clock moved backwards")},
		&mockEncoder{code: "x"},
	)
	rec := do(router, http.MethodPost, "/api/v1/generate")
	assert.Equal(t, http.StatusInternalServerError, rec.Code)

	var body map[string]any
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
	assert.Contains(t, body, "error")
}

func TestGenerate_EncoderError_Returns500(t *testing.T) {
	router := newRouterWithMocks(t,
		&mockGenerator{id: 42},
		&mockEncoder{err: errors.New("encode failed")},
	)
	rec := do(router, http.MethodPost, "/api/v1/generate")
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

// --- тесты метода ---

func TestGenerate_WrongMethod_GET_Returns405(t *testing.T) {
	rec := do(newRouter(t), http.MethodGet, "/api/v1/generate")
	assert.Equal(t, http.StatusMethodNotAllowed, rec.Code)
}

func TestGenerate_WrongMethod_PUT_Returns405(t *testing.T) {
	rec := do(newRouter(t), http.MethodPut, "/api/v1/generate")
	assert.Equal(t, http.StatusMethodNotAllowed, rec.Code)
}

func TestGenerate_WrongMethod_DELETE_Returns405(t *testing.T) {
	rec := do(newRouter(t), http.MethodDelete, "/api/v1/generate")
	assert.Equal(t, http.StatusMethodNotAllowed, rec.Code)
}

// --- тесты GET /health ---

func TestHealth_StatusOK(t *testing.T) {
	rec := do(newRouter(t), http.MethodGet, "/health")
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestHealth_BodyContainsStatus(t *testing.T) {
	rec := do(newRouter(t), http.MethodGet, "/health")

	var body map[string]string
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
	assert.Equal(t, "ok", body["status"])
}

func TestHealth_WrongMethod_Returns405(t *testing.T) {
	rec := do(newRouter(t), http.MethodPost, "/health")
	assert.Equal(t, http.StatusMethodNotAllowed, rec.Code)
}

// --- тесты 404 ---

func TestUnknownRoute_Returns404(t *testing.T) {
	rec := do(newRouter(t), http.MethodGet, "/api/v1/nonexistent")
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestUnknownRoute_BodyContainsError(t *testing.T) {
	rec := do(newRouter(t), http.MethodGet, "/api/v1/nonexistent")

	var body map[string]string
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
	assert.Contains(t, body, "error")
}
