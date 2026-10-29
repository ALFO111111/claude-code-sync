package engine

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/ALFO111111/claude-code-sync/internal/config"
	"github.com/ALFO111111/claude-code-sync/internal/fsutil"
	"github.com/ALFO111111/claude-code-sync/internal/i18n"
)

const (
	ConflictFile     = "file"
	ConflictSettings = "settings"
	ConflictMCP      = "mcp"
	ConflictTasks    = "tasks"
)

// Conflict 是一个尚未决定采用哪一边的同步项。正文分文件保存，列表不用读大文件。
type Conflict struct {
	ID         string    `json:"id"`
	Key        string    `json:"key"`
	Kind       string    `json:"kind"`
	Category   string    `json:"category"`
	Rel        string    `json:"rel"`
	LocalPath  string    `json:"localPath"`
	LocalHash  string    `json:"localHash"`
	RemoteETag string    `json:"remoteEtag,omitempty"`
	BaseHash   string    `json:"baseHash,omitempty"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

type ConflictBlobs struct {
	Base   []byte
	Local  []byte
	Remote []byte
}

func conflictsDir() string { return filepath.Join(config.Dir(), "conflicts") }

func conflictID(key string) string {
	h := sha256.Sum256([]byte(key))
	return hex.EncodeToString(h[:])
}

func conflictPath(id string) string { return filepath.Join(conflictsDir(), id) }

func saveConflict(c *Conflict, blobs ConflictBlobs) error {
	if c.ID == "" {
		c.ID = conflictID(c.Key)
	}
	now := time.Now()
	if c.CreatedAt.IsZero() {
		if old, err := LoadConflict(c.ID); err == nil {
			c.CreatedAt = old.CreatedAt
		} else {
			c.CreatedAt = now
		}
	}
	c.UpdatedAt = now
	dir := conflictPath(c.ID)
	for name, data := range map[string][]byte{"base": blobs.Base, "local": blobs.Local, "remote": blobs.Remote} {
		if err := fsutil.WriteFileAtomic(filepath.Join(dir, name), data); err != nil {
			return err
		}
	}
	meta, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return fsutil.WriteFileAtomic(filepath.Join(dir, "meta.json"), meta)
}

func LoadConflict(id string) (*Conflict, error) {
	if !conflictIDPattern.MatchString(id) {
		return nil, errors.New(i18n.T("非法冲突 ID"))
	}
	data, err := os.ReadFile(filepath.Join(conflictPath(id), "meta.json"))
	if err != nil {
		return nil, err
	}
	var c Conflict
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, err
	}
	return &c, nil
}

func ReadConflictBlobs(id string) (*ConflictBlobs, error) {
	if !conflictIDPattern.MatchString(id) {
		return nil, errors.New(i18n.T("非法冲突 ID"))
	}
	dir := conflictPath(id)
	var out ConflictBlobs
	for name, dst := range map[string]*[]byte{"base": &out.Base, "local": &out.Local, "remote": &out.Remote} {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return nil, err
		}
		*dst = data
	}
	return &out, nil
}

func ListConflicts() ([]Conflict, error) {
	entries, err := os.ReadDir(conflictsDir())
	if errors.Is(err, fs.ErrNotExist) {
		return []Conflict{}, nil
	}
	if err != nil {
		return nil, err
	}
	out := []Conflict{}
	for _, ent := range entries {
		if !ent.IsDir() {
			continue
		}
		c, err := LoadConflict(ent.Name())
		if err == nil {
			out = append(out, *c)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UpdatedAt.After(out[j].UpdatedAt) })
	return out, nil
}

func deleteConflict(id string) error {
	if !conflictIDPattern.MatchString(id) {
		return errors.New(i18n.T("非法冲突 ID"))
	}
	err := os.RemoveAll(conflictPath(id))
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	return err
}
