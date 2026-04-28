package middleware_test

import (
	"bytes"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"urlshortener/internal/gateway/middleware"
)

// --- helpers ---

func handlerOK() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}

func handlerPanic(v any) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic(v)
	})
}

func captureLog(f func()) string {
	var buf bytes.Buffer
	old := log.Writer()
	oldFlags := log.Flags()
	log.SetOutput(&buf)
	log.SetFlags(0)
	defer func() {
		log.SetOutput(old)
		log.SetFlags(oldFlags)
	}()
	f()
	return buf.String()
}

// =============================================================================
// RequestID
// =============================================================================

func TestRequestID_SetsHeaderInResponse(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	middleware.RequestID(handlerOK()).ServeHTTP(rec, req)

	assert.NotEmpty(t, rec.Header().Get("X-Request-ID"), "response must contain X-Request-ID")
}

func TestRequestID_ReusesClientHeader(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Request-ID", "my-trace-id")
	rec := httptest.NewRecorder()
	middleware.RequestID(handlerOK()).ServeHTTP(rec, req)

	assert.Equal(t, "my-trace-id", rec.Header().Get("X-Request-ID"))
}

func TestRequestID_GeneratesUniqueIDs(t *testing.T) {
	ids := make(map[string]struct{}, 100)
	for i := 0; i < 100; i++ {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		middleware.RequestID(handlerOK()).ServeHTTP(rec, req)
		id := rec.Header().Get("X-Request-ID")
		require.NotEmpty(t, id)
		_, dup := ids[id]
		assert.False(t, dup, "duplicate request ID: %s", id)
		ids[id] = struct{}{}
	}
}

func TestRequestID_PassesIDToContext(t *testing.T) {
	var ctxID string
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctxID = middleware.GetRequestID(r.Context())
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	middleware.RequestID(next).ServeHTTP(rec, req)

	assert.NotEmpty(t, ctxID, "request ID must be available in context")
	assert.Equal(t, rec.Header().Get("X-Request-ID"), ctxID)
}

func TestGetRequestID_EmptyContext_ReturnsEmptyString(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	// Контекст без RequestID — должен вернуть пустую строку, не паниковать
	id := middleware.GetRequestID(req.Context())
	assert.Empty(t, id)
}

// =============================================================================
// RateLimiter
// =============================================================================

func TestRateLimiter_AllowsRequestsWithinLimit(t *testing.T) {
	// burst=5 — первые 5 запросов должны пройти
	limiter := middleware.RateLimiter(10, 5)
	router := limiter(handlerOK())

	for i := 0; i < 5; i++ {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "192.168.1.1:1234"
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code, "request %d must pass", i+1)
	}
}

func TestRateLimiter_BlocksRequestsOverBurst(t *testing.T) {
	// rps=1, burst=1 — второй запрос подряд должен быть заблокирован
	limiter := middleware.RateLimiter(1, 1)
	router := limiter(handlerOK())

	makeReq := func() int {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "10.0.0.1:9999"
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec.Code
	}

	first := makeReq()
	assert.Equal(t, http.StatusOK, first, "first request must pass")

	second := makeReq()
	assert.Equal(t, http.StatusTooManyRequests, second, "second request must be rate limited")
}

func TestRateLimiter_DifferentIPsHaveIndependentLimits(t *testing.T) {
	// rps=1, burst=1 — разные IP не влияют друг на друга
	limiter := middleware.RateLimiter(1, 1)
	router := limiter(handlerOK())

	makeReq := func(ip string) int {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = ip + ":1234"
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec.Code
	}

	assert.Equal(t, http.StatusOK, makeReq("1.1.1.1"))
	assert.Equal(t, http.StatusOK, makeReq("2.2.2.2")) // другой IP — свой бюджет
}

func TestRateLimiter_BlockedResponse_ContentType(t *testing.T) {
	limiter := middleware.RateLimiter(1, 1)
	router := limiter(handlerOK())

	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "5.5.5.5:80"
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code == http.StatusTooManyRequests {
			assert.Contains(t, rec.Body.String(), "rate limit exceeded")
			return
		}
	}
	t.Fatal("rate limit was never triggered")
}

func TestRateLimiter_RefillsTokensOverTime(t *testing.T) {
	// rps=100, burst=1 — после ожидания токен восстанавливается
	limiter := middleware.RateLimiter(100, 1)
	router := limiter(handlerOK())

	ip := "3.3.3.3:80"
	makeReq := func() int {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = ip
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec.Code
	}

	assert.Equal(t, http.StatusOK, makeReq())              // первый — ок
	assert.Equal(t, http.StatusTooManyRequests, makeReq()) // второй — лимит

	time.Sleep(15 * time.Millisecond) // ждём пополнения токена

	assert.Equal(t, http.StatusOK, makeReq()) // после паузы — снова ок
}

// =============================================================================
// Logger
// =============================================================================

func TestLogger_LogsMethod(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/shorten", nil)
	rec := httptest.NewRecorder()

	output := captureLog(func() {
		middleware.Logger(handlerOK()).ServeHTTP(rec, req)
	})

	assert.Contains(t, output, "POST")
}

func TestLogger_LogsPath(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/shorten", nil)
	rec := httptest.NewRecorder()

	output := captureLog(func() {
		middleware.Logger(handlerOK()).ServeHTTP(rec, req)
	})

	assert.Contains(t, output, "/api/v1/shorten")
}

func TestLogger_LogsStatusCode(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	output := captureLog(func() {
		middleware.Logger(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		})).ServeHTTP(rec, req)
	})

	assert.Contains(t, output, "404")
}

func TestLogger_LogsRequestID(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	// Оборачиваем Logger в RequestID чтобы ID был в контексте
	chain := middleware.RequestID(middleware.Logger(handlerOK()))

	output := captureLog(func() {
		chain.ServeHTTP(rec, req)
	})

	requestID := rec.Header().Get("X-Request-ID")
	assert.Contains(t, output, requestID, "log must contain request ID")
}

// =============================================================================
// Recover
// =============================================================================

func TestRecover_NoPanic_PassesThrough(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	middleware.Recover(handlerOK()).ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestRecover_Panic_Returns500(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	captureLog(func() {
		middleware.Recover(handlerPanic("boom")).ServeHTTP(rec, req)
	})
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestRecover_Panic_DoesNotPropagate(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	require.NotPanics(t, func() {
		captureLog(func() {
			middleware.Recover(handlerPanic("critical error")).ServeHTTP(rec, req)
		})
	})
}

func TestRecover_Panic_LogsError(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	output := captureLog(func() {
		middleware.Recover(handlerPanic("db connection lost")).ServeHTTP(rec, req)
	})

	assert.Contains(t, output, "db connection lost")
}
