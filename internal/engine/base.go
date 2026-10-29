package engine

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/ALFO111111/claude-code-sync/internal/config"
	"github.com/ALFO111111/claude-code-sync/internal/crypt"
	"github.com/ALFO111111/claude-code-sync/internal/fsutil"
	"github.com/ALFO111111/claude-code-sync/internal/state"
)

func baseDir() string { return filepath.Join(config.Dir(), "base") }

func basePath(key string) string {
	h := sha256.Sum256([]byte(key))
	return filepath.Join(baseDir(), hex.EncodeToString(h[:]))
}

// loadBase 读取上次共同版本。BaseHash 对不上时当作没有 base，绝不拿错误版本做三方合并。
func loadBase(key string, st *state.FileState) ([]byte, bool) {
	if st == nil || st.BaseHash == "" {
		return nil, false
	}
	data, err := os.ReadFile(basePath(key))
	if err != nil || crypt.SHA256Hex(data) != st.BaseHash {
		return nil, false
	}
	return data, true
}

func saveBase(key string, data []byte) (string, error) {
	hash := crypt.SHA256Hex(data)
	if err := fsutil.WriteFileAtomic(basePath(key), data); err != nil {
		return "", err
	}
	return hash, nil
}

func removeBase(key string) error {
	err := os.Remove(basePath(key))
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	return err
}
