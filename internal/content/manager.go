package content

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
)

type Manager struct {
	publishedPath string
	draftPath     string

	mu        sync.RWMutex
	published Site
	draft     Site
}

func NewManager(dataDir string, defaults Site) (*Manager, error) {
	manager := &Manager{
		publishedPath: filepath.Join(dataDir, "site.published.json"),
		draftPath:     filepath.Join(dataDir, "site.draft.json"),
		published:     cloneSite(defaults),
		draft:         cloneSite(defaults),
	}

	published, err := loadSiteFile(manager.publishedPath)
	if err != nil {
		return nil, err
	}
	if published.Brand != "" {
		manager.published = published
		manager.draft = cloneSite(published)
	}

	draft, err := loadSiteFile(manager.draftPath)
	if err != nil {
		return nil, err
	}
	if draft.Brand != "" {
		manager.draft = draft
	}

	if err := manager.ensureFiles(context.Background()); err != nil {
		return nil, err
	}

	return manager, nil
}

func (m *Manager) Published() Site {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return cloneSite(m.published)
}

func (m *Manager) Draft() Site {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return cloneSite(m.draft)
}

func (m *Manager) SaveDraft(ctx context.Context, site Site) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.draft = cloneSite(site)
	return saveSiteFile(ctx, m.draftPath, m.draft)
}

func (m *Manager) Publish(ctx context.Context) (Site, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.published = cloneSite(m.draft)
	if err := saveSiteFile(ctx, m.publishedPath, m.published); err != nil {
		return Site{}, err
	}
	return cloneSite(m.published), nil
}

func (m *Manager) ensureFiles(ctx context.Context) error {
	if err := saveSiteFile(ctx, m.publishedPath, m.published); err != nil {
		return err
	}
	return saveSiteFile(ctx, m.draftPath, m.draft)
}

func loadSiteFile(path string) (Site, error) {
	payload, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Site{}, nil
		}
		return Site{}, err
	}

	if len(payload) == 0 {
		return Site{}, nil
	}

	var site Site
	if err := json.Unmarshal(payload, &site); err != nil {
		return Site{}, err
	}

	return site, nil
}

func saveSiteFile(ctx context.Context, path string, site Site) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	payload, err := json.MarshalIndent(site, "", "  ")
	if err != nil {
		return err
	}

	tmp, err := os.CreateTemp(filepath.Dir(path), "site-*.json")
	if err != nil {
		return err
	}

	tmpName := tmp.Name()
	if _, err := tmp.Write(payload); err != nil {
		tmp.Close()
		_ = os.Remove(tmpName)
		return err
	}
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		_ = os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return err
	}

	if err := os.Rename(tmpName, path); err != nil {
		_ = os.Remove(path)
		if secondErr := os.Rename(tmpName, path); secondErr != nil {
			_ = os.Remove(tmpName)
			return secondErr
		}
	}

	return nil
}

func cloneSite(site Site) Site {
	payload, err := json.Marshal(site)
	if err != nil {
		return site
	}

	var clone Site
	if err := json.Unmarshal(payload, &clone); err != nil {
		return site
	}
	return clone
}
