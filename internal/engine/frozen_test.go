package engine

import (
	"encoding/hex"
	"testing"

	"github.com/ALFO111111/agent-sync/internal/crypt"
)

// 改名（claude-code-sync → agent-sync）不能改变云端数据格式：同一口令派生出同一密钥、
// 分叉会话算出同一个 ID，否则旧数据解不开、新旧版本之间会多出重复会话。下面的值是改名前的版本算出来的。
func TestFrozenAcrossRename(t *testing.T) {
	if got := hex.EncodeToString(crypt.DeriveKey("correct horse", crypt.KeySalt)); got != "d125c3f3cc89b7699726b1aed183c611bc9bb60e1c86a142a9bb0bb613e37623" {
		t.Errorf("密钥派生变了: %s", got)
	}
	if crypt.KeycheckPlain != "claude-code-sync keycheck v1" {
		t.Errorf("口令校验明文变了: %q", crypt.KeycheckPlain)
	}
	if got := forkUUID("11111111-2222-3333-4444-555555555555", []byte("remote")); got != "f18e77cd-d265-4a6d-a8ab-9ac62537afdb" {
		t.Errorf("Claude 分叉会话 ID 变了: %s", got)
	}
	if got := codexForkID("019a0000-1111-7222-8333-444455556666", "dev-b"); got != "019a0000-1111-7883-be86-a8f3b0d9f4e0" {
		t.Errorf("Codex 分叉会话 ID 变了: %s", got)
	}
}
