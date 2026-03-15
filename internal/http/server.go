package http

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

	"github.com/robotjoosen/ai-assistant-driver/internal/controller"
	"github.com/robotjoosen/ai-assistant-driver/internal/tts"
)

type Server struct {
	httpServer     *http.Server
	listener       net.Listener
	host           string
	port           int
	files          map[string]string
	mu             sync.RWMutex
	started        bool
	boundIP        string
	apiKey         string
	controller     *controller.Controller
	ttsSynthesizer tts.Synthesizer
}

func NewServer(host string, port int) *Server {
	return &Server{
		host:  host,
		port:  port,
		files: make(map[string]string),
	}
}

func (s *Server) Start() error {
	if s.started {
		return nil
	}

	if s.port == 0 {
		s.port = 8080
	}

	addr := fmt.Sprintf("0.0.0.0:%d", s.port)

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", addr, err)
	}
	s.listener = listener

	if s.host == "" || s.host == "0.0.0.0" {
		s.boundIP = detectOutboundIP()
	} else {
		s.boundIP = s.host
	}

	mux := s.newMux()

	s.httpServer = &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	go func() {
		slog.Info("starting HTTP server", "addr", addr)
		if err := s.httpServer.Serve(listener); err != nil && err != http.ErrServerClosed {
			slog.Error("HTTP server error", "error", err)
		}
	}()

	s.started = true
	slog.Info("HTTP server started", "url", s.URL())

	return nil
}

func (s *Server) newMux() *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc("/api/tts/audio/", s.handleAudio)
	mux.HandleFunc("/api/status", s.handleStatus)
	mux.HandleFunc("/api/tts", s.handleTTS)
	mux.HandleFunc("/api/listen", s.handleListen)
	mux.HandleFunc("/api/lights", s.handleLights)

	return mux
}

func detectOutboundIP() string {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		slog.Warn("failed to detect outbound IP, using 127.0.0.1", "error", err)
		return "127.0.0.1"
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)
	ip := localAddr.IP.String()
	slog.Info("detected outbound IP for HTTP server", "ip", ip)
	return ip
}

func (s *Server) SetController(ctrl *controller.Controller) {
	s.controller = ctrl
}

func (s *Server) SetTTSSynthesizer(synth tts.Synthesizer) {
	s.ttsSynthesizer = synth
}

func (s *Server) SetAPIKey(key string) {
	s.apiKey = key
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

	url := fmt.Sprintf("%s/api/tts/audio/%s", s.URL(), filename)

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
