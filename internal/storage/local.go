package storage

// 本地文件夹实现：测试用，也可以指向网盘客户端的同步目录。

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/ALFO111111/agent-sync/internal/fsutil"
)

type localStorage struct{ root string }

func (s *localStorage) path(key string) string { return filepath.Join(s.root, filepath.FromSlash(key)) }

// etagOf 用 大小+mtime 当 etag，避免每次 List 都把所有文件全量读一遍算 md5（挂网盘时会非常慢）。
func etagOf(info fs.FileInfo) string {
	return fmt.Sprintf("%X-%X", info.Size(), info.ModTime().UnixNano())
}

func (s *localStorage) List(ctx context.Context, prefix string) ([]Object, error) {
	var out []Object
	err := filepath.WalkDir(s.root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return nil
			}
			return err
		}
		if d.IsDir() || strings.HasPrefix(d.Name(), fsutil.TempPrefix) {
			return nil
		}
		rel, _ := filepath.Rel(s.root, p)
		key := filepath.ToSlash(rel)
		if !strings.HasPrefix(key, prefix) {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		out = append(out, Object{Key: key, ETag: etagOf(info), Size: info.Size(), Modified: info.ModTime()})
		return nil
	})
	return out, err
}

func (s *localStorage) Get(ctx context.Context, key string) ([]byte, string, error) {
	data, err := os.ReadFile(s.path(key))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, "", ErrNotFound
	}
	if err != nil {
		return nil, "", err
	}
	info, err := os.Stat(s.path(key))
	if err != nil {
		return nil, "", err
	}
	return data, etagOf(info), nil
}

func (s *localStorage) Put(ctx context.Context, key string, data []byte) (string, error) {
	if err := fsutil.WriteFileAtomic(s.path(key), data); err != nil {
		return "", err
	}
	info, err := os.Stat(s.path(key))
	if err != nil {
		return "", err
	}
	return etagOf(info), nil
}

func (s *localStorage) Stat(ctx context.Context, key string) (Object, error) {
	info, err := os.Stat(s.path(key))
	if errors.Is(err, fs.ErrNotExist) {
		return Object{}, ErrNotFound
	}
	if err != nil {
		return Object{}, err
	}
	return Object{Key: key, ETag: etagOf(info), Size: info.Size(), Modified: info.ModTime()}, nil
}
