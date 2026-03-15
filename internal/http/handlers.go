package http

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"path/filepath"

	"github.com/robotjoosen/ai-assistant-driver/internal/controller"
)

func (s *Server) handleAudio(w http.ResponseWriter, r *http.Request) {
	filename := filepath.Base(r.URL.Path)

	s.mu.RLock()
	filePath, exists := s.files[filename]
	s.mu.RUnlock()

	if !exists {
		writeProblem(w, http.StatusNotFound, "Not Found", "Audio file not found")
		return
	}

	w.Header().Set("Content-Type", "audio/wav")
	w.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=%s", filename))
	http.ServeFile(w, r, filePath)
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeProblem(w, http.StatusMethodNotAllowed, "Method Not Allowed", "GET method is required")
		return
	}

	ctrl := s.controller
	if ctrl == nil {
		writeProblem(w, http.StatusInternalServerError, "Internal Server Error", "Controller not configured")
		return
	}

	status := map[string]string{
		"phase":        ctrl.CurrentPhase().String(),
		"transcript":   ctrl.CurrentTranscript(),
		"llm_response": ctrl.CurrentLLMResponse(),
	}

	s.writeJSON(w, status)
}

func (s *Server) handleTTS(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeProblem(w, http.StatusMethodNotAllowed, "Method Not Allowed", "POST method is required")
		return
	}

	ctrl := s.controller
	if ctrl == nil {
		writeProblem(w, http.StatusInternalServerError, "Internal Server Error", "Controller not configured")
		return
	}

	phase := ctrl.CurrentPhase()
	if phase != controller.PhaseIdle && phase != controller.PhaseReply {
		writeProblem(w, http.StatusConflict, "Phase Conflict",
			fmt.Sprintf("Cannot trigger TTS in %s phase", phase),
			map[string]any{"currentPhase": phase.String(), "allowedPhases": []string{"idle", "reply"}})
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeProblem(w, http.StatusBadRequest, "Bad Request", "Failed to read request body")
		return
	}

	var req struct {
		Text string `json:"text"`
		Wait bool   `json:"wait"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		writeProblem(w, http.StatusBadRequest, "Bad Request", "Invalid JSON")
		return
	}

	if req.Text == "" {
		writeProblem(w, http.StatusBadRequest, "Bad Request", "Text is required")
		return
	}

	if err := ctrl.TriggerTTS(req.Text); err != nil {
		writeProblem(w, http.StatusInternalServerError, "Internal Server Error", fmt.Sprintf("TTS failed: %v", err))
		return
	}

	s.writeJSON(w, map[string]any{"success": true, "message": "TTS queued"})
}

func (s *Server) handleListen(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeProblem(w, http.StatusMethodNotAllowed, "Method Not Allowed", "POST method is required")
		return
	}

	ctrl := s.controller
	if ctrl == nil {
		writeProblem(w, http.StatusInternalServerError, "Internal Server Error", "Controller not configured")
		return
	}

	phase := ctrl.CurrentPhase()
	if phase != controller.PhaseIdle {
		writeProblem(w, http.StatusConflict, "Phase Conflict",
			fmt.Sprintf("Cannot start listening in %s phase", phase),
			map[string]any{"currentPhase": phase.String(), "allowedPhases": []string{"idle"}})
		return
	}

	if err := ctrl.TriggerListening(); err != nil {
		writeProblem(w, http.StatusInternalServerError, "Internal Server Error", fmt.Sprintf("Failed to start listening: %v", err))
		return
	}

	s.writeJSON(w, map[string]any{"success": true, "message": "Listening activated"})
}

func (s *Server) handleLights(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeProblem(w, http.StatusMethodNotAllowed, "Method Not Allowed", "POST method is required")
		return
	}

	ctrl := s.controller
	if ctrl == nil {
		writeProblem(w, http.StatusInternalServerError, "Internal Server Error", "Controller not configured")
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeProblem(w, http.StatusBadRequest, "Bad Request", "Failed to read request body")
		return
	}

	var req struct {
		Action     string    `json:"action"`
		EntityID   string    `json:"entity_id"`
		Brightness int       `json:"brightness"`
		Color      colorJSON `json:"color"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		writeProblem(w, http.StatusBadRequest, "Bad Request", "Invalid JSON")
		return
	}

	if req.Action == "" {
		writeProblem(w, http.StatusBadRequest, "Bad Request", "Action is required")
		return
	}

	color := controller.RGB{
		R: req.Color.R,
		G: req.Color.G,
		B: req.Color.B,
	}

	if err := ctrl.ControlLight(req.Action, req.EntityID, req.Brightness, color); err != nil {
		writeProblem(w, http.StatusInternalServerError, "Internal Server Error", fmt.Sprintf("Light control failed: %v", err))
		return
	}

	s.writeJSON(w, map[string]any{"success": true, "message": fmt.Sprintf("Light %s", req.Action)})
}

type colorJSON struct {
	R int `json:"r"`
	G int `json:"g"`
	B int `json:"b"`
}

func (s *Server) writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("failed to encode JSON response", "error", err)
	}
}
