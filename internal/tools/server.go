package tools

import (
	"encoding/json"
	"net/http"
)

type Server struct {
	Registry *Registry
}

type executeRequest struct {
	Name  string          `json:"name"`
	Input json.RawMessage `json:"input"`
}

type executeResponse struct {
	OK     bool   `json:"ok"`
	Output Result `json:"output"`
	Error  string `json:"error,omitempty"`
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/tools/list", s.handleList)
	mux.HandleFunc("/tools/execute", s.handleExecute)
	return mux
}

func (s *Server) handleList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	resp := map[string][]string{"tools": s.Registry.List()}
	_ = json.NewEncoder(w).Encode(resp)
}

func (s *Server) handleExecute(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var req executeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(executeResponse{OK: false, Error: err.Error()})
		return
	}
	tool := s.Registry.Get(req.Name)
	if tool == nil {
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(executeResponse{OK: false, Error: "tool not found"})
		return
	}
	res, err := tool.Run(r.Context(), req.Input)
	if err != nil {
		_ = json.NewEncoder(w).Encode(executeResponse{OK: false, Output: res, Error: err.Error()})
		return
	}
	_ = json.NewEncoder(w).Encode(executeResponse{OK: true, Output: res})
}
