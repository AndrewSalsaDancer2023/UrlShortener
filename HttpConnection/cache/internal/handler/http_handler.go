package handler

import (
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

	pair, err := utils.ReadURL[utils.URLPair](http.StatusBadRequest, w, req)
	if err != nil {
		return
	}

	urls := h.ctrl.CreateURLValuePair(pair.LongURL, pair.ShortURL)
	utils.WriteURL(&urls, http.StatusInternalServerError, w)
}

func (h *Handler) GetLongURL(w http.ResponseWriter, req *http.Request) {
	shorted_url, ok := mux.Vars(req)["shorted_url"]
	if !ok {
		utils.WriteErrorResponse(http.StatusBadRequest, ErrShortURLNotSpecified.Error(), w)
		return
	}

	longURL, err := h.ctrl.GetValueForURL(shorted_url)
	if err != nil {
		utils.WriteErrorResponse(http.StatusNotFound, err.Error(), w)
		return
	}

	utils.WriteURL(&utils.LongURL{URL: longURL}, http.StatusInternalServerError, w)
}
