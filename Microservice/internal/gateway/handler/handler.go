package handler

import (
	"context"
	"encoding/json"
	"log"
	"net/http"

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
/*
func (h *GatewayHandler) Shorten(w http.ResponseWriter, r *http.Request) {
	var data RequestData

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
		requestID := middleware.GetRequestID(r.Context())
		result, err := h.idclient.Generate(r.Context())
		if err != nil {
			//при генерации короткого URL произошла ошибка, возвращаем ее
			log.Printf("[ERROR] Status: %d | Message: %s | RequestID: %s", http.StatusBadGateway, err.Error(), requestID)
			writeError(w, http.StatusInternalServerError, "upstream error", requestID)
			return
		}

		shortURL, err = h.dbshortener.Shorten(r.Context(), result.NumericID, data.LongURL)
		if err != nil {
			log.Printf("[ERROR] Status: %d | Message: %s | RequestID: %s", http.StatusBadGateway, err.Error(), requestID)
			writeError(w, http.StatusInternalServerError, "upstream error", requestID)
			return
		}
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
*/

func (h *GatewayHandler) generateAndSaveURL(ctx context.Context, longURL, requestID string) (int64, error) {
	result, err := h.idclient.Generate(ctx)
	if err != nil {
		log.Printf("[ERROR] Status: %d | Message: %s | RequestID: %s", http.StatusBadGateway, err.Error(), requestID)
		return 0, err
	}

	// Запись в базу данных
	shortURL, err := h.dbshortener.Shorten(ctx, result.NumericID, longURL)
	if err != nil {
		log.Printf("[ERROR] Status: %d | Message: %s | RequestID: %s", http.StatusBadGateway, err.Error(), requestID)
		return 0, err
	}

	// Асинхронно или синхронно пишем в кэш. Ошибка кэша не должна прерывать флоу пользователя.
	if cacheErr := h.urlcache.SaveURLPair(ctx, shortURL, longURL); cacheErr != nil {
		log.Printf("[WARN] Failed to save URL pair to cache | Message: %s | RequestID: %s", cacheErr.Error(), requestID)
	}

	return shortURL, nil
}

func (h *GatewayHandler) Shorten(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	requestID := middleware.GetRequestID(ctx)

	var data RequestData
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		writeError(w, http.StatusBadRequest, "long url not specified", requestID)
		return
	}

	if len(data.LongURL) > maxURLLength {
		writeError(w, http.StatusBadRequest, "URL is too long", requestID)
		return
	}

	// 1. Пытаемся получить из кэша
	shortURL, err := h.urlcache.GetShortURL(ctx, 0, data.LongURL)
	if err != nil {
		// Кэш-мисс или ошибка кэша: генерируем и сохраняем новый URL
		shortURL, err = h.generateAndSaveURL(ctx, data.LongURL, requestID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "upstream error", requestID)
			return
		}
	}

	// 2. Кодируем ID в короткий код (общая логика для обоих путей)
	encoder := converter.New()
	res, err := encoder.Encode(shortURL)
	if err != nil {
		log.Printf("[ERROR] Status: %d | Message: %s | RequestID: %s", http.StatusBadGateway, err.Error(), requestID)
		writeError(w, http.StatusInternalServerError, "upstream error", requestID)
		return
	}

	// 3. Отправляем успешный ответ
	writeJSON(w, http.StatusOK, map[string]any{
		"numeric_id": shortURL,
		"short_code": res,
		"short_url":  "https://short.ly/" + res,
	})
}

/*
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
		log.Printf("[ERROR] Status: %d | Message: %s | RequestID: %s", http.StatusBadGateway, err.Error(), requestID)
		longURL, err = h.dbrestorer.Restore(r.Context(), resURL)
		if err != nil {

			writeError(w, http.StatusInternalServerError, "no associated long URL found", requestID)
			return
		}
		err := h.urlcache.SaveURLPair(r.Context(), resURL, longURL)
		if err != nil {
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
*/

func (h *GatewayHandler) GetOriginal(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	requestID := middleware.GetRequestID(ctx)

	shortCode, ok := mux.Vars(r)["shorted_url"]
	if !ok || shortCode == "" {
		writeError(w, http.StatusBadRequest, "short url not specified", requestID)
		return
	}

	// Декодируем короткую строку обратно в числовой Snowflake ID
	numericID, err := converter.New().Decode(shortCode)
	if err != nil {
		log.Printf("[ERROR] Decode failed | Message: %s | RequestID: %s", err.Error(), requestID)
		writeError(w, http.StatusBadRequest, "invalid short url format", requestID)
		return
	}

	// 1. Пробуем получить длинный URL из кэша
	longURL, err := h.urlcache.GetLongURL(ctx, numericID)
	if err != nil {
		// Кэш-мисс или сбой кэша: идем в базу данных
		longURL, err = h.dbrestorer.Restore(ctx, numericID)
		if err != nil {
			// Ссылки нет и в базе данных — это клиентская ошибка 404, а не 500
			writeError(w, http.StatusNotFound, "no associated long URL found", requestID)
			return
		}

		// Ссылка нашлась в БД, асинхронно или фоном обновляем кэш, чтобы не тормозить ответ
		if cacheErr := h.urlcache.SaveURLPair(ctx, numericID, longURL); cacheErr != nil {
			log.Printf("[WARN] Failed to save URL pair to cache | Message: %s | RequestID: %s", cacheErr.Error(), requestID)
		}
	}

	// 2. Единая точка отправки успешного ответа
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
