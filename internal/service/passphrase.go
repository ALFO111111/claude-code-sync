package service

import (
	"context"
	"errors"

	"github.com/ALFO111111/agent-sync/internal/crypt"
	"github.com/ALFO111111/agent-sync/internal/engine"
	"github.com/ALFO111111/agent-sync/internal/i18n"
)

// 同步口令：修改（所有设备一起换），以及别的设备改过之后本机换上新口令。

// ChangePassphrase 修改同步口令：云端全部文件用新口令重新加密（文件多时要几分钟）。
// 改完后其他设备会停下来，提示输入新口令。中途失败可以原样再来一次。
func ChangePassphrase(ctx context.Context, oldPass, newPass string, progress func(done, total int)) error {
	if newPass == "" {
		return errors.New(i18n.T("口令不能为空"))
	}
	return engine.ChangeKey(ctx, crypt.DeriveKey(oldPass, crypt.KeySalt), crypt.DeriveKey(newPass, crypt.KeySalt), progress)
}

// AdoptPassphrase 别的设备改过口令后，在本机输入新口令（会先和云端核对）。
func AdoptPassphrase(ctx context.Context, pass string) error {
	if pass == "" {
		return errors.New(i18n.T("口令不能为空"))
	}
	return engine.AdoptKey(ctx, crypt.DeriveKey(pass, crypt.KeySalt))
}

// PassphraseMatches 本机口令和云端是否一致（不一致 = 别的设备改过口令，本机要用 AdoptPassphrase 换上新的）。
func PassphraseMatches(ctx context.Context) (bool, error) { return engine.KeyMatchesCloud(ctx) }
