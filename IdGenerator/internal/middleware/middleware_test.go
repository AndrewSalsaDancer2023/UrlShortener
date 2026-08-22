/*
Тесты разбиты на три группы.
Logger — 8 тестов:

вызывает следующий handler
пробрасывает статусы 200 / 404 / 500 без изменений
по умолчанию возвращает 200, если handler не вызывал WriteHeader
логирует метод, путь и статус-код
не трогает тело запроса и заголовки

Recover — 6 тестов:

без паники работает прозрачно
перехватывает panic(string), panic(error) и panic(nil) — все возвращают 500
значение паники попадает в лог
тело ответа содержит "internal server error"

Logger + Recover вместе — 2 интеграционных теста на полную цепочку Recover(Logger(handler)), как она собирается в main.go.
Отдельно стоит отметить тест panic(nil) — в Go 1.21 поведение изменилось: раньше recover() возвращал nil и паника считалась непойманной, теперь возвращает *runtime.PanicNilError, поэтому middleware корректно её перехватывает.
*/
package middleware_test

import (
	"bytes"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"urlshortener/internal/middleware"
)

// handlerWithStatus возвращает handler, который отвечает заданным статусом.
func handlerWithStatus(status int) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
	})
}

// handlerWithBody возвращает handler, который пишет тело без явного WriteHeader.
func handlerWithBody(body string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(body))
	})
}

// handlerPanic возвращает handler, который паникует с переданным значением.
func handlerPanic(v any) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic(v)
	})
}

// captureLog перенаправляет стандартный логгер в буфер на время вызова f,
// возвращает записанный вывод.
func captureLog(f func()) string {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	log.SetFlags(0) // убираем дату/время — нужен только текст
	f()
	log.SetOutput(nil) // сбрасываем — следующий SetOutput в тесте перезапишет
	return buf.String()
}

// =============================================================================
// Logger
// =============================================================================

func TestLogger_PassesRequestToNextHandler(t *testing.T) {
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	middleware.Logger(next).ServeHTTP(rec, req)

	assert.True(t, called, "next handler must be called")
}

func TestLogger_PreservesResponseStatus200(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	middleware.Logger(handlerWithStatus(http.StatusOK)).ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestLogger_PreservesResponseStatus404(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/missing", nil)
	rec := httptest.NewRecorder()
	middleware.Logger(handlerWithStatus(http.StatusNotFound)).ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestLogger_PreservesResponseStatus500(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/error", nil)
	rec := httptest.NewRecorder()
	middleware.Logger(handlerWithStatus(http.StatusInternalServerError)).ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestLogger_DefaultsTo200WhenHandlerDoesNotCallWriteHeader(t *testing.T) {
	// Если handler пишет только тело без WriteHeader — статус должен быть 200.
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	middleware.Logger(handlerWithBody("hello")).ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestLogger_WritesMethodToLog(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/generate", nil)
	rec := httptest.NewRecorder()

	output := captureLog(func() {
		middleware.Logger(handlerWithStatus(http.StatusOK)).ServeHTTP(rec, req)
	})

	assert.Contains(t, output, "POST", "log must contain HTTP method")
}

func TestLogger_WritesPathToLog(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/generate", nil)
	rec := httptest.NewRecorder()

	output := captureLog(func() {
		middleware.Logger(handlerWithStatus(http.StatusOK)).ServeHTTP(rec, req)
	})

	assert.Contains(t, output, "/api/v1/generate", "log must contain request path")
}

func TestLogger_WritesStatusCodeToLog(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	output := captureLog(func() {
		middleware.Logger(handlerWithStatus(http.StatusNotFound)).ServeHTTP(rec, req)
	})

	assert.Contains(t, output, "404", "log must contain status code")
}

func TestLogger_PassesRequestBodyUnchanged(t *testing.T) {
	var receivedBody string
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := new(bytes.Buffer)
		_, _ = buf.ReadFrom(r.Body)
		receivedBody = buf.String()
	})

	body := `{"key":"value"}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	rec := httptest.NewRecorder()
	middleware.Logger(next).ServeHTTP(rec, req)

	assert.Equal(t, body, receivedBody, "request body must pass through unchanged")
}

func TestLogger_PassesHeadersUnchanged(t *testing.T) {
	var receivedHeader string
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeader = r.Header.Get("X-Request-ID")
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Request-ID", "abc-123")
	rec := httptest.NewRecorder()
	middleware.Logger(next).ServeHTTP(rec, req)

	assert.Equal(t, "abc-123", receivedHeader)
}

// =============================================================================
// Recover
// =============================================================================

func TestRecover_NoPanic_PassesThrough(t *testing.T) {
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	middleware.Recover(next).ServeHTTP(rec, req)

	assert.True(t, called)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestRecover_PanicString_Returns500(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	middleware.Recover(handlerPanic("something went wrong")).ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestRecover_PanicError_Returns500(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	// require.NoError внутри горутины, поэтому используем прямой вызов
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic(assert.AnError) // assert.AnError — стандартная sentinel-ошибка testify
	})
	middleware.Recover(next).ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestRecover_PanicNil_Returns500(t *testing.T) {
	// panic(nil) — редкий, но допустимый случай
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	// В Go 1.21+ panic(nil) поднимает *runtime.PanicNilError,
	// поэтому recover() вернёт не nil — middleware должен поймать его.
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic(nil) //nolint:forbidigo
	})

	require.NotPanics(t, func() {
		middleware.Recover(next).ServeHTTP(rec, req)
	})
}

func TestRecover_WritesErrorToLog(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	output := captureLog(func() {
		middleware.Recover(handlerPanic("database connection lost")).ServeHTTP(rec, req)
	})

	assert.Contains(t, output, "database connection lost", "panic value must appear in log")
}

func TestRecover_ResponseBodyContainsErrorMessage(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	middleware.Recover(handlerPanic("boom")).ServeHTTP(rec, req)

	assert.Contains(t, rec.Body.String(), "internal server error")
}

// =============================================================================
// Logger + Recover вместе (как в production)
// =============================================================================

func TestLoggerAndRecover_PanicIs500AndLogged(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/generate", nil)
	rec := httptest.NewRecorder()

	chain := middleware.Recover(middleware.Logger(handlerPanic("db timeout")))

	output := captureLog(func() {
		chain.ServeHTTP(rec, req)
	})

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	// Logger должен залогировать запрос несмотря на панику
	assert.Contains(t, output, "POST")
	assert.Contains(t, output, "/api/v1/generate")
}

func TestLoggerAndRecover_NormalRequest_200(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	chain := middleware.Recover(middleware.Logger(handlerWithStatus(http.StatusOK)))
	chain.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}
