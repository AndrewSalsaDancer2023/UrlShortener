package handler

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"

	mux "github.com/gorilla/mux"
	cr "urlshortener.com/cache/internal/controller"
	"urlshortener.com/gateway/pkg/config"
	"urlshortener.com/utils"
)

var ErrShortURLNotSpecified = errors.New("short url doesn't specified")

type Handler struct {
	ctrl *cr.CacheController
}

func New(ctrl *cr.CacheController) *Handler {
	return &Handler{ctrl}
}

func SetupRouter(h *Handler) *mux.Router {
	router := mux.NewRouter()
	cfg := config.GetConfig()

	log.Println("Setup GET handler on " + cfg.OriginalURLPath)
	log.Println("Setup POST handler on " + cfg.ShortURLPath)

	router.HandleFunc(cfg.OriginalURLPath, h.GetLongURL).Methods("GET")
	router.HandleFunc(cfg.ShortURLPath, h.CreateURLPair).Methods("POST")

	return router
}

func (h *Handler) CreateURLPair(w http.ResponseWriter, req *http.Request) {

	var record utils.URLPair
	if err := json.NewDecoder(req.Body).Decode(&record); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(err.Error()))
		return
	}

	urls := h.ctrl.CreateURLValuePair(record.LongURL, record.ShortURL)
	if err := json.NewEncoder(w).Encode(urls); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		return
	}

}

func (h *Handler) GetLongURL(w http.ResponseWriter, req *http.Request) {

	shorted_url, ok := mux.Vars(req)["shorted_url"]
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(ErrShortURLNotSpecified.Error()))
		return
	}

	val, err := h.ctrl.GetValueForURL(shorted_url)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(err.Error()))
		return
	}

	resp := utils.ShortURL{URL: val}
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		return
	}
}
