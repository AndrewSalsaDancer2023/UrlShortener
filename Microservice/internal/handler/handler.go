/*
package handler

import (

	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"

	"urlshortener/internal/service"

)

// IDHandler обрабатывает HTTP-запросы генерации ID.
// Зависит только от сервисного слоя — не знает про генератор и EventBus.

	type IDHandler struct {
		svc *service.IDService
	}

	func New(svc *service.IDService) *IDHandler {
		return &IDHandler{svc: svc}
	}

// NewRouter создаёт и возвращает настроенный gorilla/mux роутер.
// Маршруты регистрируются здесь — роутер передаётся в http.Server снаружи.

	func (h *IDHandler) NewRouter() *mux.Router {
		r := mux.NewRouter()

		// Версионированный API-префикс
		api := r.PathPrefix("/api/v1").Subrouter()
		api.HandleFunc("/generate", h.Generate).Methods(http.MethodPost)

		// Служебные маршруты
		r.HandleFunc("/health", h.Health).Methods(http.MethodGet)

		// Глобальный 404 и 405
		r.NotFoundHandler = http.HandlerFunc(h.NotFound)
		r.MethodNotAllowedHandler = http.HandlerFunc(h.MethodNotAllowed)

		return r
	}

// Generate обрабатывает POST /api/v1/generate
//
// Response 200:
//
//	{"numeric_id": 7816251636736237568, "short_code": "2qF4xK8"}

	func (h *IDHandler) Generate(w http.ResponseWriter, r *http.Request) {
		result, err := h.svc.Generate()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, result)
	}

// Health обрабатывает GET /health

	func (h *IDHandler) Health(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}

// NotFound — обработчик несуществующих маршрутов.

	func (h *IDHandler) NotFound(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusNotFound, "route not found")
	}

// MethodNotAllowed — обработчик неверного HTTP-метода.

	func (h *IDHandler) MethodNotAllowed(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}

// --- helpers ---

	type errorResponse struct {
		Error string `json:"error"`
	}

	func writeJSON(w http.ResponseWriter, status int, v any) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(v)
	}

	func writeError(w http.ResponseWriter, status int, msg string) {
		writeJSON(w, status, errorResponse{Error: msg})
	}
*/
package handler

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"

	"urlshortener/internal/service"
)

// IDHandler обрабатывает HTTP-запросы генерации ID.
// Зависит только от сервисного слоя — не знает про генератор и EventBus.
type IDHandler struct {
	svc *service.IDService
}

func New(svc *service.IDService) *IDHandler {
	return &IDHandler{svc: svc}
}

// NewRouter создаёт и возвращает настроенный gorilla/mux роутер.
// Маршруты регистрируются здесь — роутер передаётся в http.Server снаружи.
func (h *IDHandler) NewRouter() *mux.Router {
	r := mux.NewRouter()

	// Версионированный API-префикс
	api := r.PathPrefix("/api/v1").Subrouter()
	api.HandleFunc("/generate", h.Generate).Methods(http.MethodGet)

	// Служебные маршруты
	r.HandleFunc("/health", h.Health).Methods(http.MethodGet)

	// Глобальный 404 и 405
	r.NotFoundHandler = http.HandlerFunc(h.NotFound)
	r.MethodNotAllowedHandler = http.HandlerFunc(h.MethodNotAllowed)

	return r
}

// Generate обрабатывает POST /api/v1/generate
//
// Response 200:
//
//	{"numeric_id": 7816251636736237568, "short_code": "2qF4xK8"}
func (h *IDHandler) Generate(w http.ResponseWriter, r *http.Request) {
	result, err := h.svc.Generate()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// Health обрабатывает GET /health
func (h *IDHandler) Health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// NotFound — обработчик несуществующих маршрутов.
func (h *IDHandler) NotFound(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotFound, "route not found")
}

// MethodNotAllowed — обработчик неверного HTTP-метода.
func (h *IDHandler) MethodNotAllowed(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusMethodNotAllowed, "method not allowed")
}

// --- helpers ---

type errorResponse struct {
	Error string `json:"error"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, errorResponse{Error: msg})
}
