package server

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"huntera-bot/pkg/bot"
)

//go:embed index.html
var defaultIndexHTML []byte

type StatusProvider interface {
	GetAllStatuses() []bot.AccountStatus
}

type Server struct {
	port     int
	provider StatusProvider
}

func NewServer(port int, provider StatusProvider) *Server {
	return &Server{
		port:     port,
		provider: provider,
	}
}

func (s *Server) Start() error {
	mux := http.NewServeMux()

	// 1. API Status com CORS
	mux.HandleFunc("/api/status", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		statuses := s.provider.GetAllStatuses()

		resp := map[string]any{
			"serverTime": time.Now().UnixMilli(),
			"accounts":   statuses,
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(resp)
	})

	// 2. Servir Dashboard estático
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		diskFile := filepath.Join("docs", "index.html")
		if _, err := os.Stat(diskFile); err == nil {
			http.ServeFile(w, r, diskFile)
			return
		}

		// Fallback para index.html embutido
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(defaultIndexHTML)
	})

	addr := fmt.Sprintf(":%d", s.port)
	log.Printf("[Web Server] Servidor HTTP nativo iniciado em http://0.0.0.0%s", addr)
	log.Printf("[Web Server] API pública: http://0.0.0.0%s/api/status", addr)

	srv := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	return srv.ListenAndServe()
}
