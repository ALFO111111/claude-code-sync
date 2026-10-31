package service

import (
	"bytes"
	"context"
	"unicode/utf8"

	"github.com/ALFO111111/agent-sync/internal/config"
	"github.com/ALFO111111/agent-sync/internal/engine"
)

// 冲突页：两台电脑把同一个全局文件（规则、计划、技能、settings 等）改成了不同的样子，
// 自动合并不了，等用户决定。冲突本身由 engine 在同步时记下，本机文件在此期间保持原样可用。

// ConflictItem 冲突列表里的一条。时间用毫秒（Wails 绑定不认 time.Time）。
type ConflictItem struct {
	ID        string `json:"id"`
	Tool      string `json:"tool"`
	Kind      string `json:"kind"`     // file | settings | mcp | tasks
	Category  string `json:"category"` // 见 config.Content*
	Rel       string `json:"rel"`
	LocalPath string `json:"localPath"`
	Time      int64  `json:"time"`
	TimeText  string `json:"timeText"`
}

// ConflictContent 一个冲突的三份内容：共同版本、本机、另一台。
type ConflictContent struct {
	Conflict ConflictItem `json:"conflict"`
	Base     string       `json:"base"`
	Local    string       `json:"local"`
	Remote   string       `json:"remote"`
	HasBase  bool         `json:"hasBase"`
	Binary   bool         `json:"binary"`   // 不是文本：只能选一边
	TooLarge bool         `json:"tooLarge"` // 太大只显示开头：不能手工合并
	CanBoth  bool         `json:"canBoth"`  // 能"两份都留"（结构化配置和全局规则不行）
}

func conflictItem(c engine.Conflict) ConflictItem {
	return ConflictItem{ID: c.ID, Tool: c.Tool, Kind: c.Kind, Category: c.Category, Rel: c.Rel, LocalPath: c.LocalPath,
		Time: c.UpdatedAt.UnixMilli(), TimeText: fmtTime(c.UpdatedAt)}
}

func ListConflicts() ([]ConflictItem, error) {
	list, err := engine.ListConflicts()
	if err != nil {
		return nil, err
	}
	out := []ConflictItem{}
	for _, c := range list {
		out = append(out, conflictItem(c))
	}
	return out, nil
}

func ReadConflict(id string) (*ConflictContent, error) {
	c, err := engine.LoadConflict(id)
	if err != nil {
		return nil, err
	}
	blobs, err := engine.ReadConflictBlobs(id)
	if err != nil {
		return nil, err
	}
	out := &ConflictContent{
		Conflict: conflictItem(*c),
		HasBase:  c.BaseHash != "",
		CanBoth:  c.Category != config.ContentRules && (c.Kind == engine.ConflictFile || c.Kind == engine.ConflictTasks),
	}
	for _, b := range [][]byte{blobs.Base, blobs.Local, blobs.Remote} {
		if sniff := truncUTF8(b, 8192); !utf8.Valid(sniff) || bytes.IndexByte(sniff, 0) >= 0 {
			out.Binary = true
			return out, nil
		}
		if len(b) > maxPreview {
			out.TooLarge = true
		}
	}
	out.Base = string(truncUTF8(blobs.Base, maxPreview))
	out.Local = string(truncUTF8(blobs.Local, maxPreview))
	out.Remote = string(truncUTF8(blobs.Remote, maxPreview))
	return out, nil
}

// ResolveConflict choice：local 保留本机 / remote 用另一台的 / both 两份都留 / merged 用手工合并的 merged。
func ResolveConflict(ctx context.Context, id, choice, merged string) error {
	return engine.ResolveConflict(ctx, id, choice, []byte(merged))
}

// RestoreContentItem 本机删掉的全局文件，从云端拉回来（之后照常同步）。
func RestoreContentItem(ctx context.Context, category, rel string) error {
	return engine.RestoreContentItem(ctx, category, rel)
}

// RestoreToolContentItem 恢复指定工具的本机已删除文件。
func RestoreToolContentItem(ctx context.Context, tool, category, rel string) error {
	return engine.RestoreToolContentItem(ctx, tool, category, rel)
}
