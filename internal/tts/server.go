package tts

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Server struct {
	httpServer *http.Server
	listener   net.Listener
	host       string
	port       int
	files      map[string]string
	mu         sync.RWMutex
	started    bool
	boundIP    string
}

func NewServer(host string, port int) *Server {
	return &Server{
		host:  host,
		port:  port,
		files: make(map[string]string),
	}
}

func getOutboundIP() (string, error) {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return "", err
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)
	return localAddr.IP.String(), nil
}

func (s *Server) Start() error {
	if s.started {
		return nil
	}

	if s.port == 0 {
		s.port = 8080
	}

	// Listen on all interfaces
	addr := fmt.Sprintf("0.0.0.0:%d", s.port)

	var err error
	s.listener, err = net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", addr, err)
	}

	// Get the actual IP we're bound to for URL generation
	if s.host == "" || s.host == "0.0.0.0" {
		ip, err := getOutboundIP()
		if err != nil {
			slog.Warn("failed to detect outbound IP, using 127.0.0.1", "error", err)
			s.boundIP = "127.0.0.1"
		} else {
			s.boundIP = ip
			slog.Info("detected outbound IP for TTS server", "ip", s.boundIP)
		}
	} else {
		s.boundIP = s.host
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/audio/", s.handleAudio)

	s.httpServer = &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	go func() {
		slog.Info("starting TTS HTTP server", "addr", addr)
		if err := s.httpServer.Serve(s.listener); err != nil && err != http.ErrServerClosed {
			slog.Error("TTS HTTP server error", "error", err)
		}
	}()

	s.started = true
	slog.Info("TTS HTTP server started", "addr", s.URL())

	return nil
}

func (s *Server) handleAudio(w http.ResponseWriter, r *http.Request) {
	filename := filepath.Base(r.URL.Path)

	s.mu.RLock()
	filePath, exists := s.files[filename]
	s.mu.RUnlock()

	if !exists {
		http.NotFound(w, r)
		return
	}

	headers := w.Header()
	headers.Set("Content-Type", "audio/wav")
	headers.Set("Content-Disposition", fmt.Sprintf("inline; filename=%s", filename))

	http.ServeFile(w, r, filePath)
}

func (s *Server) ServeWAV(filePath string) (string, func(), error) {
	if !s.started {
		if err := s.Start(); err != nil {
			return "", nil, fmt.Errorf("failed to start HTTP server: %w", err)
		}
	}

	filename := filepath.Base(filePath)

	s.mu.Lock()
	s.files[filename] = filePath
	s.mu.Unlock()

	url := fmt.Sprintf("%s/audio/%s", s.URL(), filename)

	cleanup := func() {
		s.mu.Lock()
		delete(s.files, filename)
		s.mu.Unlock()

		if err := os.Remove(filePath); err != nil {
			slog.Warn("failed to remove TTS file", "path", filePath, "error", err)
		}
	}

	return url, cleanup, nil
}

func (s *Server) URL() string {
	return fmt.Sprintf("http://%s:%d", s.boundIP, s.port)
}

func (s *Server) Close() error {
	if s.httpServer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return s.httpServer.Shutdown(ctx)
	}
	return nil
}
