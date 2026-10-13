package service

import (
	"os"
	"strings"
	"testing"

	"github.com/ALFO111111/claude-code-sync/internal/logging"
)

// 日志：读尾部带路径；清空不存在的日志不报错；超长时从整行开始截。
func TestReadAndClearLog(t *testing.T) {
	t.Setenv("CLAUDE_CODE_SYNC_HOME", t.TempDir())
	if err := ClearLog(); err != nil {
		t.Errorf("日志不存在时清空不应报错: %v", err)
	}
	logging.New(true).Printf("第一行")
	logging.New(true).Printf("第二行")
	v, err := ReadLog(1 << 20)
	if err != nil || !strings.Contains(v.Text, "第一行") || v.Path == "" {
		t.Fatalf("ReadLog: %v %+v", err, v)
	}
	// 只读最后一点点：第一行被切掉，但不会出现半行
	v, _ = ReadLog(len("第二行") + 40)
	if strings.Contains(v.Text, "第一行") || !strings.HasSuffix(v.Text, "第二行\n") {
		t.Errorf("截尾应从整行开始: %q", v.Text)
	}

	if err := ClearLog(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(logging.Path()); !os.IsNotExist(err) {
		t.Error("清空后日志文件应不存在")
	}
}
