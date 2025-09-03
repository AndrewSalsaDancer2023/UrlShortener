package handler

import (
	"log"
	"net/http"

	mux "github.com/gorilla/mux"
	cr "urlshortener.com/engine/internal/controller"
	"urlshortener.com/gateway/pkg/config"
	"urlshortener.com/utils"
)

type Handler struct {
	ctrl *cr.Controller
}

func New(ctrl *cr.Controller) *Handler {
	return &Handler{ctrl}
}

func SetupRouter(h *Handler) *mux.Router {
	router := mux.NewRouter()
	cfg := config.GetConfig()

	log.Println("Starting POST handler on: " + cfg.ShortURLPath)

	router.HandleFunc(cfg.ShortURLPath, h.CreateShortURL).Methods("POST")

	return router
}

func (h *Handler) CreateShortURL(w http.ResponseWriter, req *http.Request) {
	val, err := h.ctrl.CreateRandomValue()
	if err != nil {
		utils.WriteErrorResponse(http.StatusInternalServerError, err.Error(), w)
		return
	}

	resp := utils.ShortURL{URL: val}

	utils.WriteURL(&resp, http.StatusInternalServerError, w)
}
