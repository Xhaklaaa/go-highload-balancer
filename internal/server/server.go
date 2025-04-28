package server

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	"github.com/xhaklaaa/go-highload-balancer/internal/api/handler"
	"github.com/xhaklaaa/go-highload-balancer/internal/balancer/interfaces"
	"github.com/xhaklaaa/go-highload-balancer/internal/limiter"
	"github.com/xhaklaaa/go-highload-balancer/internal/limiter/store"
	"github.com/xhaklaaa/go-highload-balancer/internal/logger"
)

type Server struct {
	router           *mux.Router
	balancer         interfaces.Balancer
	proxyHandler     http.Handler
	port             int
	logger           logger.Logger
	rateLimiter      *limiter.TokenBucket
	httpServer       *http.Server
	rateLimiterStore limiter.ConfigStore
}

func NewServer(lb interfaces.Balancer, proxyHandler http.Handler, port int, log logger.Logger, rateLimiter *limiter.TokenBucket, store limiter.ConfigStore) *Server {
	router := mux.NewRouter()
	s := &Server{
		router:           router,
		balancer:         lb,
		proxyHandler:     proxyHandler,
		port:             port,
		logger:           log,
		rateLimiter:      rateLimiter,
		rateLimiterStore: store,
	}
	s.setupRoutes()
	return s
}

func (s *Server) setupRoutes() {
	adminRouter := s.router.PathPrefix("/admin").Subrouter()
	adminRouter.HandleFunc("/backend-status", s.handleBackendStatus).Methods("POST")

	apiRouter := s.router.PathPrefix("/api/v1").Subrouter()
	apiRouter.Use(jsonMiddleware)
	s.setupAPIRoutes(apiRouter)

	s.router.PathPrefix("/").Handler(s.proxyHandler)
}

func (s *Server) setupAPIRoutes(router *mux.Router) {
	memStore := store.NewInMemoryStore(limiter.RateConfig{
		Capacity:   100,
		RefillRate: 10,
	})

	clientHandler := handler.NewClientHandler(memStore, s.logger)
	clientHandler.RegisterRoutes(router)
}

func (s *Server) handleBackendStatus(w http.ResponseWriter, r *http.Request) {
	var request struct {
		URL   string `json:"url"`
		Alive bool   `json:"alive"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	s.balancer.MarkBackendStatus(request.URL, request.Alive)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Backend status updated"))
}

func jsonMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		next.ServeHTTP(w, r)
	})
}

func (s *Server) Start() error {
	s.setupRoutes()
	s.router.Walk(func(route *mux.Route, router *mux.Router, ancestors []*mux.Route) error {
		path, _ := route.GetPathTemplate()
		s.logger.Infof("Registered route: %s", path)
		return nil
	})

	s.httpServer = &http.Server{
		Addr:         ":" + strconv.Itoa(s.port),
		Handler:      s.router,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  15 * time.Second,
	}

	s.logger.Infof("Starting server on port %d", s.port)
	return s.httpServer.ListenAndServe()
}

func (s *Server) Stop(ctx context.Context) error {
	if err := s.rateLimiter.Stop(); err != nil {
		s.logger.Errorf("Rate limiter shutdown error: %v", err)
	}
	return s.httpServer.Shutdown(ctx)
}
