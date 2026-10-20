package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

type Store struct {
	dir string
	mu  sync.Mutex
}

func New(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	return &Store{dir: dir}, nil
}

type cursorFile struct {
	Cursors map[string]string `json:"cursors"`
}

func (s *Store) GetPromotionCursor(platform, feedKey string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := s.readCursors()
	if err != nil {
		return "", err
	}
	key := platform + ":" + feedKey
	if data.Cursors == nil {
		return "", nil
	}
	return data.Cursors[key], nil
}

func (s *Store) SetPromotionCursor(platform, feedKey, itemID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := s.readCursors()
	if err != nil {
		return err
	}
	if data.Cursors == nil {
		data.Cursors = make(map[string]string)
	}
	data.Cursors[platform+":"+feedKey] = itemID
	return s.writeCursors(data)
}

func (s *Store) HasSeenFirstMessage(userID, guildID string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	m, err := s.readFirstMessages()
	if err != nil {
		return false, err
	}
	_, ok := m[userID+":"+guildID]
	return ok, nil
}

func (s *Store) MarkFirstMessage(userID, guildID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	m, err := s.readFirstMessages()
	if err != nil {
		return err
	}
	if m == nil {
		m = make(map[string]bool)
	}
	m[userID+":"+guildID] = true
	return s.writeFirstMessages(m)
}

func (s *Store) cursorsPath() string  { return filepath.Join(s.dir, "cursors.json") }
func (s *Store) firstMsgPath() string { return filepath.Join(s.dir, "first_message.json") }

func (s *Store) readCursors() (*cursorFile, error) {
	var data cursorFile
	if err := s.readJSON(s.cursorsPath(), &data); err != nil {
		return nil, err
	}
	return &data, nil
}

func (s *Store) writeCursors(data *cursorFile) error {
	return s.writeJSON(s.cursorsPath(), data)
}

func (s *Store) readFirstMessages() (map[string]bool, error) {
	m := make(map[string]bool)
	if err := s.readJSON(s.firstMsgPath(), &m); err != nil {
		return nil, err
	}
	return m, nil
}

func (s *Store) writeFirstMessages(m map[string]bool) error {
	return s.writeJSON(s.firstMsgPath(), m)
}

func (s *Store) readJSON(path string, v any) error {
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if len(b) == 0 {
		return nil
	}
	if err := json.Unmarshal(b, v); err != nil {
		return fmt.Errorf("parse %s: %w", path, err)
	}
	return nil
}

func (s *Store) writeJSON(path string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}
