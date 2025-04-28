package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/gorilla/mux"
	"github.com/xhaklaaa/go-highload-balancer/internal/limiter"
	"github.com/xhaklaaa/go-highload-balancer/internal/logger"
)

type ClientHandler struct {
	store  limiter.ConfigStore
	logger logger.Logger
}

func NewClientHandler(store limiter.ConfigStore, logger logger.Logger) *ClientHandler {
	return &ClientHandler{
		store:  store,
		logger: logger,
	}
}

type ClientRequest struct {
	ID       string  `json:"client_id" validate:"required,alphanum"`
	Capacity int64   `json:"capacity" validate:"required,gt=0"`
	Rate     float64 `json:"rate_per_sec" validate:"required,gt=0"`
}

type ClientResponse struct {
	ID       string  `json:"client_id"`
	Capacity int64   `json:"capacity"`
	Rate     float64 `json:"rate_per_sec"`
}

type ErrorResponse struct {
	Error struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

const (
	contentTypeJSON = "application/json"
	apiVersion      = "2023-07-01"
)

func (h *ClientHandler) RegisterRoutes(router *mux.Router) {
	router.HandleFunc("/clients", h.createClient).Methods("POST")
	router.HandleFunc("/clients/{id}", h.getClient).Methods("GET")
	router.HandleFunc("/clients/{id}", h.updateClient).Methods("PUT")
	router.HandleFunc("/clients/{id}", h.deleteClient).Methods("DELETE")
}

func (h *ClientHandler) createClient(w http.ResponseWriter, r *http.Request) {
	var req ClientRequest
	if err := decodeAndValidate(r, &req); err != nil {
		h.respondError(w, err.Code, err.Message)
		return
	}

	config := limiter.RateConfig{
		Capacity:   req.Capacity,
		RefillRate: req.Rate,
	}

	if err := h.store.UpsertConfig(r.Context(), req.ID, config); err != nil {
		h.logger.Errorf("Failed to create client: %v", err)
		h.respondError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	h.respondJSON(w, http.StatusCreated, ClientResponse{
		ID:       req.ID,
		Capacity: config.Capacity,
		Rate:     config.RefillRate,
	})
}

func (h *ClientHandler) getClient(w http.ResponseWriter, r *http.Request) {
	clientID := mux.Vars(r)["id"]

	config, exists, err := h.store.GetConfig(r.Context(), clientID)
	if err != nil {
		h.logger.Errorf("Failed to get client config: %v", err)
		h.respondError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	if !exists {
		h.respondError(w, http.StatusNotFound, "client not found")
		return
	}

	h.respondJSON(w, http.StatusOK, ClientResponse{
		ID:       clientID,
		Capacity: config.Capacity,
		Rate:     config.RefillRate,
	})
}

func (h *ClientHandler) updateClient(w http.ResponseWriter, r *http.Request) {
	clientID := mux.Vars(r)["id"]

	var req ClientRequest
	if err := decodeAndValidate(r, &req); err != nil {
		h.respondError(w, err.Code, err.Message)
		return
	}

	if clientID != req.ID {
		h.respondError(w, http.StatusBadRequest, "client ID mismatch")
		return
	}

	config := limiter.RateConfig{
		Capacity:   req.Capacity,
		RefillRate: req.Rate,
	}

	if err := h.store.UpsertConfig(r.Context(), clientID, config); err != nil {
		h.logger.Errorf("Failed to update client: %v", err)
		h.respondError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	h.respondJSON(w, http.StatusOK, ClientResponse{
		ID:       clientID,
		Capacity: config.Capacity,
		Rate:     config.RefillRate,
	})
}

func (h *ClientHandler) deleteClient(w http.ResponseWriter, r *http.Request) {
	clientID := mux.Vars(r)["id"]

	if err := h.store.DeleteConfig(r.Context(), clientID); err != nil {
		h.logger.Errorf("Failed to delete client: %v", err)
		h.respondError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

type validationError struct {
	Code    int
	Message string
}

func decodeAndValidate(r *http.Request, v interface{}) *validationError {
	defer r.Body.Close()

	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		return &validationError{
			Code:    http.StatusBadRequest,
			Message: "invalid JSON format",
		}
	}

	validate := validator.New()
	if err := validate.Struct(v); err != nil {
		var errMsgs []string
		for _, err := range err.(validator.ValidationErrors) {
			errMsgs = append(errMsgs, fmt.Sprintf(
				"field %s: %s",
				err.Field(),
				err.Tag(),
			))
		}
		return &validationError{
			Code:    http.StatusUnprocessableEntity,
			Message: strings.Join(errMsgs, "; "),
		}
	}

	return nil
}

func (h *ClientHandler) respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", contentTypeJSON)
	w.Header().Set("Api-Version", apiVersion)
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(data); err != nil {
		h.logger.Errorf("Failed to encode response: %v", err)
	}
}

func (h *ClientHandler) respondError(w http.ResponseWriter, code int, message string) {
	resp := ErrorResponse{}
	resp.Error.Code = code
	resp.Error.Message = message

	h.respondJSON(w, code, resp)
}
