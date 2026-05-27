package handler

import (
	"encoding/json"
	"net/http"

	"log"

	"github.com/gorilla/mux"

	//	"urlshortener/internal/gateway/client"
	converter "urlshortener/internal/base62"
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

// GatewayHandler обрабатывает входящие запросы и проксирует их к upstream.
type GatewayHandler struct {
	idclient    domain.IDGeneratorInterface
	dbshortener domain.URLShortenerInterface
	dbrestorer  domain.URLRestorerInterface
	urlcache    domain.URLCacheInterface
}

func New(idclnt domain.IDGeneratorInterface,
	shrtclnt domain.URLShortenerInterface,
	rstclnt domain.URLRestorerInterface,
	cache domain.URLCacheInterface) *GatewayHandler {
	return &GatewayHandler{idclient: idclnt, dbshortener: shrtclnt, dbrestorer: rstclnt, urlcache: cache}
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

	shortURL, err := h.urlcache.GetShortURL(r.Context(), 0, data.LongURL)
	if err != nil {
		//произошла ошибка или запрашиваемый  LongURL не найден в кэше
		//генерируем короткий ID для ссылки
		requestID := middleware.GetRequestID(r.Context())
		result, err := h.idclient.Generate(r.Context())
		if err != nil {
			//при генерации короткого URL произошла ошибка, возвращаем ее
			log.Printf("[ERROR] Status: %d | Message: %s | RequestID: %s", http.StatusBadGateway, err.Error(), requestID)
			writeError(w, http.StatusInternalServerError, "upstream error", requestID)
			return
		}
		//пишем в базу пару короткий URL - длинный URL
		shortURL, err = h.dbshortener.Shorten(r.Context(), result.NumericID, data.LongURL)
		if err != nil {
			//при записи в базу произошла ошибка, возвращаем её
			log.Printf("[ERROR] Status: %d | Message: %s | RequestID: %s", http.StatusBadGateway, err.Error(), requestID)
			writeError(w, http.StatusInternalServerError, "upstream error", requestID)
			return
		}
		//пищем пару URL в кэш
		err = h.urlcache.SaveURLPair(r.Context(), shortURL, data.LongURL)
		if err != nil {
			//при записи URL в кэш произошла ошибка, логируем её
			log.Printf("[ERROR] Status: %d | Message: %s | RequestID: %s", http.StatusBadGateway, err.Error(), requestID)
		}

		encoder := converter.New()
		res, err := encoder.Encode(shortURL)
		if err != nil {
			requestID := middleware.GetRequestID(r.Context())
			log.Printf("[ERROR] Status: %d | Message: %s | RequestID: %s", http.StatusBadGateway, err.Error(), requestID)
			writeError(w, http.StatusInternalServerError, "upstream error", requestID)
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"numeric_id": shortURL,
			"short_code": res,
			"short_url":  "https://short.ly/" + res,
		})
	}

	// LongURL найден в кэше, нужно преобразовать его в строку и вернуть
	encoder := converter.New()
	res, err := encoder.Encode(shortURL)
	if err != nil {
		requestID := middleware.GetRequestID(r.Context())
		log.Printf("[ERROR] Status: %d | Message: %s | RequestID: %s", http.StatusBadGateway, err.Error(), requestID)
		writeError(w, http.StatusInternalServerError, "upstream error", requestID)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"numeric_id": shortURL,
		"short_code": res,
		"short_url":  "https://short.ly/" + res,
	})
}

func (h *GatewayHandler) GetOriginal(w http.ResponseWriter, r *http.Request) {
	shortURL, ok := mux.Vars(r)["shorted_url"]
	requestID := middleware.GetRequestID(r.Context())
	if !ok {
		writeError(w, http.StatusBadRequest, "short url not specified", requestID)
		return
	}

	resURL, err := converter.New().Decode(shortURL)
	if err != nil {
		log.Printf("[ERROR] Status: %d | Message: %s | RequestID: %s", http.StatusBadGateway, err.Error(), requestID)
		writeError(w, http.StatusInternalServerError, "upstream error", requestID)
		return
	}

	//Пробуем получить длинный URL из кеша
	longURL, err := h.urlcache.GetLongURL(r.Context(), resURL)
	if err != nil {
		// не удалось достать длинный URL из кеша, попробуем в базе
		log.Printf("[ERROR] Status: %d | Message: %s | RequestID: %s", http.StatusBadGateway, err.Error(), requestID)
		longURL, err = h.dbrestorer.Restore(r.Context(), resURL)
		if err != nil {
			//в базе длинного URL также нет, сообщаем об ошибке
			writeError(w, http.StatusInternalServerError, "no associated long URL found", requestID)
			return
		}
		//в базе нашелся длинный URL, пробуем записать его в кеш
		err := h.urlcache.SaveURLPair(r.Context(), resURL, longURL)
		if err != nil {
			//при записи в кеш возникла ошибка, логируем её
			log.Printf("[ERROR] Status: %d | Message: %s | RequestID: %s", http.StatusBadGateway, err.Error(), requestID)
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"url": longURL,
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"url": longURL,
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
