package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

type SessionStore struct {
	mu       sync.Mutex
	path     string
	sessions map[string]SessionState
}

type SessionState struct {
	ID      string `json:"id"`
	Dir     string `json:"dir,omitempty"`
	UseLast bool   `json:"use_last,omitempty"`
}

func NewSessionStore(dataDir string) (*SessionStore, error) {
	path := filepath.Join(dataDir, "sessions.json")
	store := &SessionStore{path: path, sessions: map[string]SessionState{}}
	if err := store.load(); err != nil {
		return nil, err
	}
	return store, nil
}

func (s *SessionStore) GetSessionID(key string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.sessions[key].ID
}

func (s *SessionStore) GetState(key string) (SessionState, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	state, ok := s.sessions[key]
	return state, ok
}

func (s *SessionStore) GetDir(key string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.sessions[key].Dir
}

func (s *SessionStore) SetSessionID(key, value string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	state := s.sessions[key]
	state.ID = value
	s.sessions[key] = state
	return s.save()
}

func (s *SessionStore) SetDir(key, dir string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	state := s.sessions[key]
	state.Dir = dir
	s.sessions[key] = state
	return s.save()
}

func (s *SessionStore) SetUseLast(key string, value bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	state := s.sessions[key]
	state.UseLast = value
	s.sessions[key] = state
	return s.save()
}

func (s *SessionStore) load() error {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var sessions map[string]SessionState
	if err := json.Unmarshal(data, &sessions); err == nil {
		s.sessions = sessions
		return nil
	} else {
		var legacy map[string]string
		if err := json.Unmarshal(data, &legacy); err == nil {
			converted := make(map[string]SessionState, len(legacy))
			for key, value := range legacy {
				converted[key] = SessionState{ID: value}
			}
			s.sessions = converted
			return nil
		}
		return err
	}
}

func (s *SessionStore) save() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s.sessions, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0o644)
}
