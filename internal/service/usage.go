package service

import (
	"context"

	"github.com/ALFO111111/agent-sync/internal/engine"
)

// CloudUsage 云端占了多少空间（列一遍云端文件，不下载内容）。
func CloudUsage(ctx context.Context) (*engine.Usage, error) {
	e, err := engine.New(true)
	if err != nil {
		return nil, err
	}
	return e.Usage(ctx)
}
