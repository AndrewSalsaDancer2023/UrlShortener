package handler

import (
	"context"
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

	router.HandleFunc(cfg.OriginalURLPath, h.GetLongURL).Methods("GET")
	router.HandleFunc(cfg.ShortURLPath, h.CreateShortURL).Methods("POST")

	return router
}

func (h *Handler) CreateShortURL(w http.ResponseWriter, r *http.Request) {
	longURL, err := utils.ReadURL[utils.LongURL](http.StatusBadRequest, w, r)
	if err != nil {
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	shortURL, err := h.ctrl.CreateShortURL(ctx, longURL.URL)
	if err != nil {
		utils.WriteErrorResponse(http.StatusInternalServerError, err.Error(), w)
		return
	}

	utils.WriteURL(&shortURL, http.StatusInternalServerError, w)
}

func (h *Handler) GetLongURL(w http.ResponseWriter, r *http.Request) {

	vars := mux.Vars(r)
	_, ok := vars["shorted_url"]
	if !ok {
		utils.WriteErrorResponse(http.StatusBadRequest, ErrShortURLNotSpecified.Error(), w)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	url, err := h.ctrl.GetLongURL(ctx, r.URL.Path)
	if err != nil {
		utils.WriteErrorResponse(http.StatusNotFound, err.Error(), w)
		return
	}
	http.Redirect(w, r, url.URL, http.StatusFound)
}
