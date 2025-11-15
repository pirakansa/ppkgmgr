package registry

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

type Entry struct {
	ID        string    `json:"id"`
	Source    string    `json:"source"`
	LocalPath string    `json:"local_path"`
	Digest    string    `json:"digest"`
	AddedAt   time.Time `json:"added_at"`
}

type Store struct {
	Entries []Entry `json:"entries"`
}

func Load(path string) (Store, error) {
	var store Store

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return store, nil
		}
		return store, fmt.Errorf("read registry: %w", err)
	}

	if len(data) == 0 {
		return store, nil
	}

	if err := json.Unmarshal(data, &store); err != nil {
		return store, fmt.Errorf("decode registry: %w", err)
	}

	return store, nil
}

func (s Store) Save(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create registry dir: %w", err)
	}

	sorted := append([]Entry(nil), s.Entries...)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Source < sorted[j].Source
	})

	data, err := json.MarshalIndent(Store{Entries: sorted}, "", "  ")
	if err != nil {
		return fmt.Errorf("encode registry: %w", err)
	}

	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write registry: %w", err)
	}

	return nil
}

func (s *Store) Upsert(entry Entry) {
	for i, existing := range s.Entries {
		if existing.Source == entry.Source {
			if entry.ID == "" {
				entry.ID = existing.ID
			}
			s.Entries[i] = entry
			return
		}
	}
	s.Entries = append(s.Entries, entry)
}

func (s *Store) GetBySource(source string) (Entry, bool) {
	for _, entry := range s.Entries {
		if entry.Source == source {
			return entry, true
		}
	}
	return Entry{}, false
}

func (s *Store) RemoveByID(id string) (Entry, bool) {
	for i, entry := range s.Entries {
		if entry.ID == id {
			s.Entries = append(s.Entries[:i], s.Entries[i+1:]...)
			return entry, true
		}
	}
	return Entry{}, false
}

func (s *Store) RemoveBySource(source string) (Entry, bool) {
	for i, entry := range s.Entries {
		if entry.Source == source {
			s.Entries = append(s.Entries[:i], s.Entries[i+1:]...)
			return entry, true
		}
	}
	return Entry{}, false
}
