package http

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
)

type ProblemResponse struct {
	Type   string         `json:"type"`
	Title  string         `json:"title"`
	Status int            `json:"status"`
	Detail string         `json:"detail"`
	Extra  map[string]any `json:"-"`
}

func (p ProblemResponse) MarshalJSON() ([]byte, error) {
	m := map[string]any{
		"type":   p.Type,
		"title":  p.Title,
		"status": p.Status,
		"detail": p.Detail,
	}
	for k, v := range p.Extra {
		m[k] = v
	}
	return json.Marshal(m)
}

func writeProblem(w http.ResponseWriter, status int, title, detail string, extra ...map[string]any) {
	problem := ProblemResponse{
		Type:   "",
		Title:  title,
		Status: status,
		Detail: detail,
	}
	if len(extra) > 0 {
		problem.Extra = extra[0]
	}

	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(problem); err != nil {
		slog.Error("failed to encode problem response", "error", err)
	}
}

func writeProblemf(w http.ResponseWriter, status int, title, detailFmt string, args ...any) {
	writeProblem(w, status, title, fmt.Sprintf(detailFmt, args...))
}
