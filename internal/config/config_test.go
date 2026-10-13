package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ALFO111111/claude-code-sync/internal/storage"
)

// 旧版（阿里云 OSS 专用 SDK）写的 config.json，读进来自动变成 S3 兼容配置，指向同一个 bucket。
func TestLoadMigratesOldOSSConfig(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CLAUDE_CODE_SYNC_HOME", dir)
	old := `{
  "deviceId": "abc",
  "deviceName": "pc",
  "storage": {"type": "oss", "region": "cn-shanghai", "bucket": "my-bucket",
              "accessKeyId": "ak", "accessKeySecret": "sk", "prefix": "v1/"}
}`
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(old), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	s := cfg.Storage
	if s.Type != storage.TypeS3 || s.Provider != storage.ProviderAliyun || s.Bucket != "my-bucket" ||
		s.Prefix != "v1/" || s.ResolvedEndpoint() != "https://s3.oss-cn-shanghai.aliyuncs.com" {
		t.Errorf("迁移结果不对: %+v，endpoint %s", s, s.ResolvedEndpoint())
	}
}
