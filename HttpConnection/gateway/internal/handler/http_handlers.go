package handler

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"time"

	mux "github.com/gorilla/mux"
	gateway "urlshortener.com/gateway/internal/controller"
	"urlshortener.com/gateway/pkg/config"
	"urlshortener.com/utils"
)

var ErrShortURLNotSpecified = errors.New("short url doesn't specified")

type Handler struct {
	ctrl *gateway.Controller
}

func New(ctrl *gateway.Controller) *Handler {
	return &Handler{ctrl}
}

func (h *Handler) SetupRouter() *mux.Router {
	router := mux.NewRouter()
	cfg := config.GetConfig()

	log.Println("Setup GET handler on: " + cfg.OriginalURLPath)

	log.Println("Setup POST handler on: " + cfg.ShortURLPath)

	router.HandleFunc(cfg.OriginalURLPath, h.GetOriginalURL).Methods("GET")
	router.HandleFunc(cfg.ShortURLPath, h.CreateShortURL).Methods("POST")

	return router
}

func (h *Handler) CreateShortURL(w http.ResponseWriter, r *http.Request) {

	var LongURL utils.LongURL
	if err := json.NewDecoder(r.Body).Decode(&LongURL); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(err.Error()))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()

	shortURL, err := h.ctrl.CreateShortURL(ctx, LongURL.URL)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		return
	}

	if err := json.NewEncoder(w).Encode(shortURL); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		return
	}
}

func (h *Handler) GetOriginalURL(w http.ResponseWriter, r *http.Request) {

	vars := mux.Vars(r)
	_, ok := vars["shorted_url"]
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(ErrShortURLNotSpecified.Error()))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()

	url, err := h.ctrl.GetOriginalURL(ctx, r.URL.Path)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(err.Error()))
		return
	}
	http.Redirect(w, r, url.URL, http.StatusFound)
}
