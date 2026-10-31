package service

import (
	"bytes"
	"errors"
	"os"

	"github.com/ALFO111111/agent-sync/internal/logging"
)

type LogView struct {
	Path string `json:"path"`
	Text string `json:"text"`
}

// ReadLog 读日志尾部（最多 max 字节，防止文件太大卡界面）。
func ReadLog(max int) (*LogView, error) {
	v := &LogView{Path: logging.Path()}
	data, err := os.ReadFile(v.Path)
	if errors.Is(err, os.ErrNotExist) {
		return v, nil
	}
	if err != nil {
		return nil, err
	}
	if len(data) > max {
		// 从下一个整行开始，避免第一行从汉字中间切开
		data = data[len(data)-max:]
		if i := bytes.IndexByte(data, '\n'); i >= 0 {
			data = data[i+1:]
		}
	}
	v.Text = string(data)
	return v, nil
}

// ClearLog 删除日志；本来就没有也算成功。
func ClearLog() error {
	err := os.Remove(logging.Path())
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}
