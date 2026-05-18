package handler

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/gorilla/mux"

	//	"urlshortener/internal/gateway/client"
	"urlshortener/internal/gateway/handler/domain"
	"urlshortener/internal/gateway/middleware"
)

const (
	maxURLLength = 150
)

// расшифровывает параметр запроса, передавемый в формате: {longUrl: longURLString};
type RequestData struct {
	LongURL string `json:"longUrl"`
}

// Doer — интерфейс для подмены клиента в тестах.
// type Doer interface {
// 	Generate(ctx context.Context) (*client.GenerateResponse, error)
// }

// GatewayHandler обрабатывает входящие запросы и проксирует их к upstream.
type GatewayHandler struct {
	client domain.Doer
}

func New(c domain.Doer) *GatewayHandler {
	return &GatewayHandler{client: c}
}

// NewRouter создаёт gorilla/mux роутер со всеми маршрутами Gateway.
func (h *GatewayHandler) NewRouter() *mux.Router {
	r := mux.NewRouter()

	api := r.PathPrefix("/api/v1").Subrouter()
	api.HandleFunc("/data/shorten", h.Shorten).Methods(http.MethodPost)
	api.HandleFunc("/short/{shorted_url:[a-zA-Z0-9]+}", h.GetOriginal).Methods(http.MethodGet)
	r.HandleFunc("/health", h.Health).Methods(http.MethodGet)

	r.NotFoundHandler = http.HandlerFunc(h.NotFound)
	r.MethodNotAllowedHandler = http.HandlerFunc(h.MethodNotAllowed)

	return r
}

// Shorten обрабатывает POST /api/v1/shorten.
// Запрашивает уникальный ID у upstream и возвращает short code клиенту.
//
// Response 200:
//
//	{"numeric_id": 123456, "short_code": "2qF4xK8", "short_url": "https://short.ly/2qF4xK8"}
func (h *GatewayHandler) Shorten(w http.ResponseWriter, r *http.Request) {
	var data RequestData

	// Декодируйте тело запроса в структуру
	// r.Body реализует интерфейс io.Reader, поэтому NewDecoder читает его напрямую
	err := json.NewDecoder(r.Body).Decode(&data)
	if err != nil {
		requestID := middleware.GetRequestID(r.Context())
		writeError(w, http.StatusBadRequest, "long url not specified", requestID)
		return
	}

	// Доступ к параметру через поле структуры
	if len(data.LongURL) > maxURLLength {
		requestID := middleware.GetRequestID(r.Context())
		writeError(w, http.StatusBadRequest, "URL is too long", requestID)
		return
	}

	result, err := h.client.Generate(r.Context())
	if err != nil {
		requestID := middleware.GetRequestID(r.Context())
		log.Printf("[ERROR] Status: %d | Message: %s | RequestID: %s", http.StatusBadGateway, err.Error(), requestID)
		writeError(w, http.StatusBadGateway, "upstream error", requestID)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"numeric_id": result.NumericID,
		"short_code": result.ShortCode,
		"short_url":  "https://short.ly/" + result.ShortCode,
	})
}

func (h *GatewayHandler) GetOriginal(w http.ResponseWriter, r *http.Request) {
	shortedURL, ok := mux.Vars(r)["shorted_url"]
	if !ok {
		requestID := middleware.GetRequestID(r.Context())
		writeError(w, http.StatusBadRequest, "short url not specified", requestID)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"url": shortedURL,
	})
}

// Health возвращает статус Gateway.
func (h *GatewayHandler) Health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status":     "ok",
		"request_id": middleware.GetRequestID(r.Context()),
	})
}

// NotFound — обработчик несуществующих маршрутов.
func (h *GatewayHandler) NotFound(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotFound, "route not found", middleware.GetRequestID(r.Context()))
}

// MethodNotAllowed — обработчик неверного HTTP-метода.
func (h *GatewayHandler) MethodNotAllowed(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusMethodNotAllowed, "method not allowed", middleware.GetRequestID(r.Context()))
}

// --- helpers ---

type errorResponse struct {
	Error     string `json:"error"`
	RequestID string `json:"request_id,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg, requestID string) {
	writeJSON(w, status, errorResponse{Error: msg, RequestID: requestID})
}
