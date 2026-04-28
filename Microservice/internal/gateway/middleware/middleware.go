package middleware

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"net"
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// ctxKey — приватный тип для ключей контекста, исключает коллизии с другими пакетами.
type ctxKey string

const requestIDKey ctxKey = "request_id"

// RequestID добавляет уникальный X-Request-ID к каждому запросу и ответу.
// Если клиент уже передал заголовок — используем его, иначе генерируем новый.
// Request ID удобен для сквозной трассировки запросов через все сервисы.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-ID")
		if id == "" {
			id = generateID()
		}

		// Пробрасываем ID в контекст — downstream может его прочитать
		ctx := context.WithValue(r.Context(), requestIDKey, id)

		// Возвращаем ID клиенту в заголовке ответа
		w.Header().Set("X-Request-ID", id)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// GetRequestID извлекает Request ID из контекста.
func GetRequestID(ctx context.Context) string {
	if id, ok := ctx.Value(requestIDKey).(string); ok {
		return id
	}
	return ""
}

// RateLimiter ограничивает количество запросов с одного IP.
// Использует алгоритм token bucket из golang.org/x/time/rate.
// Каждый IP получает свой независимый лимитер.
func RateLimiter(rps int, burst int) func(http.Handler) http.Handler {
	type visitor struct {
		limiter  *rate.Limiter
		lastSeen time.Time
	}

	var (
		mu       sync.Mutex
		visitors = make(map[string]*visitor)
	)

	// Фоновая горутина удаляет записи IP, которые не обращались больше 3 минут.
	// Без этого map будет расти неограниченно.
	go func() {
		for range time.Tick(time.Minute) {
			mu.Lock()
			for ip, v := range visitors {
				if time.Since(v.lastSeen) > 3*time.Minute {
					delete(visitors, ip)
				}
			}
			mu.Unlock()
		}
	}()

	getVisitor := func(ip string) *rate.Limiter {
		mu.Lock()
		defer mu.Unlock()
		v, exists := visitors[ip]
		if !exists {
			v = &visitor{limiter: rate.NewLimiter(rate.Limit(rps), burst)}
			visitors[ip] = v
		}
		v.lastSeen = time.Now()
		return v.limiter
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip, _, err := net.SplitHostPort(r.RemoteAddr)
			if err != nil {
				ip = r.RemoteAddr
			}

			limiter := getVisitor(ip)
			if !limiter.Allow() {
				requestID := GetRequestID(r.Context())
				log.Printf("[RATE_LIMIT] ip=%s request_id=%s", ip, requestID)
				http.Error(w, `{"error":"rate limit exceeded"}`, http.StatusTooManyRequests)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// Logger логирует каждый входящий запрос с Request ID для трассировки.
func Logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &responseWriter{ResponseWriter: w, status: http.StatusOK}

		next.ServeHTTP(rw, r)

		log.Printf("[GATEWAY] request_id=%s method=%s path=%s status=%d duration=%s",
			GetRequestID(r.Context()),
			r.Method,
			r.URL.Path,
			rw.status,
			time.Since(start),
		)
	})
}

// Recover перехватывает panic и возвращает 500.
func Recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				requestID := GetRequestID(r.Context())
				log.Printf("[PANIC] request_id=%s error=%v", requestID, err)
				http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// responseWriter перехватывает статус-код для логирования.
type responseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *responseWriter) WriteHeader(status int) {
	rw.status = status
	rw.ResponseWriter.WriteHeader(status)
}

// generateID генерирует случайный hex-идентификатор для трассировки.
func generateID() string {
	return fmt.Sprintf("%016x", rand.Int63())
}
