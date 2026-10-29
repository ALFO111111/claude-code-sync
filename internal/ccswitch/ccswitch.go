// Package ccswitch 只读检测 CC Switch 的自动同步状态。
package ccswitch

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

// Status 是 CC Switch 自动同步的检测结果，值稳定，可直接供调用方判断。
type Status string

const (
	NotInstalled Status = "notInstalled" // 没有 CC Switch 配置文件
	Disabled     Status = "disabled"     // 有配置，但 WebDAV 和 S3 自动同步都没开
	Enabled      Status = "enabled"      // 至少一种自动同步已开启
	Unknown      Status = "unknown"      // 配置损坏、读取失败或无法确定用户目录
)

// Detect 检测当前用户目录下的 CC Switch 配置。
func Detect() Status {
	home, err := os.UserHomeDir()
	if err != nil {
		return Unknown
	}
	return DetectAt(home)
}

// DetectAt 检测 root 用户目录下的 <root>/.cc-switch/settings.json。
// 整个检测过程只读；配置里的其他字段（包括秘密信息）不会被解析或返回。
func DetectAt(root string) Status {
	data, err := os.ReadFile(filepath.Join(root, ".cc-switch", "settings.json"))
	if errors.Is(err, os.ErrNotExist) {
		return NotInstalled
	}
	if err != nil {
		return Unknown
	}

	var settings struct {
		WebDAVSync syncSettings `json:"webdavSync"`
		S3Sync     syncSettings `json:"s3Sync"`
	}
	if err := json.Unmarshal(data, &settings); err != nil {
		return Unknown
	}
	if settings.WebDAVSync.autoEnabled() || settings.S3Sync.autoEnabled() {
		return Enabled
	}
	return Disabled
}

// syncSettings 只声明判断所需的布尔字段，encoding/json 会忽略其余字段。
type syncSettings struct {
	Enabled  bool `json:"enabled"`
	AutoSync bool `json:"autoSync"`
}

func (s syncSettings) autoEnabled() bool {
	return s.Enabled && s.AutoSync
}
