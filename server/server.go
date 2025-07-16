package server

import (
	"context"
	"encoding/json"
	"log-processor/config"
	"log-processor/db/repo"
	"log/slog"
	"net/http"
	"strconv"
)

type Server struct {
	storage repo.IClickHouseCrudStorage
	logger  *slog.Logger
	config  *config.Config
	server  *http.Server
}

func NewServer(storage repo.IClickHouseCrudStorage, logger *slog.Logger, cfg *config.Config) *Server {
	return &Server{
		storage: storage,
		logger:  logger,
		config:  cfg,
	}
}

func (s *Server) Start(ctx context.Context) error {
	mux := http.NewServeMux()

	mux.HandleFunc("/health", s.healthHandler)
	mux.HandleFunc("/logs", s.logsHandler)
	mux.HandleFunc("/stats", s.statsHandler)

	s.server = &http.Server{
		Addr:    s.config.Server.Port,
		Handler: mux,
	}

	s.logger.Info("starting HTTP server", "port", s.config.Server.Port)
	return s.server.ListenAndServe()
}

func (s *Server) Stop(ctx context.Context) error {
	if s.server != nil {
		return s.server.Shutdown(ctx)
	}
	return nil
}

func (s *Server) healthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (s *Server) logsHandler(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 {
		limit = 100
	}

	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if offset < 0 {
		offset = 0
	}

	logs, err := s.storage.GetLogs(r.Context(), limit, offset)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(logs)
}

func (s *Server) statsHandler(w http.ResponseWriter, r *http.Request) {
	stats, err := s.storage.GetLogStats(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}
