// Package storage 云端"中转站"：统一的读写接口，以及 S3 兼容对象存储、本地文件夹两种实现。
package storage

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/ALFO111111/claude-code-sync/internal/i18n"
)

// 存储类型。
const (
	TypeS3    = "s3"    // S3 兼容对象存储（阿里云 OSS、腾讯云 COS、Cloudflare R2、AWS S3、MinIO……）
	TypeLocal = "local" // 本地文件夹（测试用）
	typeOSS   = "oss"   // 旧版：阿里云 OSS 专用 SDK，读配置时自动转成 s3 + aliyun
)

// S3 服务商。只影响 Endpoint 留空时怎么自动生成，以及界面怎么提示；真正连接都走 S3 协议。
const (
	ProviderAliyun  = "aliyun"
	ProviderTencent = "tencent"
	ProviderR2      = "r2"
	ProviderAWS     = "aws"
	ProviderMinIO   = "minio"
	ProviderOther   = "other"
)

// Config 存储配置（config.json 的 storage 字段）。
type Config struct {
	Type            string `json:"type"`               // TypeS3 或 TypeLocal
	Provider        string `json:"provider,omitempty"` // S3 服务商，见 Provider* 常量
	Endpoint        string `json:"endpoint,omitempty"` // 留空时按服务商+地域生成（aliyun/tencent/aws）
	Region          string `json:"region,omitempty"`
	Bucket          string `json:"bucket,omitempty"`
	AccessKeyID     string `json:"accessKeyId,omitempty"`
	AccessKeySecret string `json:"accessKeySecret,omitempty"`
	PathStyle       bool   `json:"pathStyle,omitempty"` // 路径风格 URL（MinIO 等自建服务通常要开），默认虚拟主机风格
	Prefix          string `json:"prefix,omitempty"`
	Path            string `json:"path,omitempty"` // TypeLocal 的文件夹
}

// Normalize 补默认前缀 v1/、保证前缀以 / 结尾，并把旧版的阿里云 OSS 配置转成 S3 兼容写法。
// 只改内存里的值；下次保存设置时才写回 config.json。转换前后指向同一个 bucket、同一批对象，云端数据不用动。
func (c *Config) Normalize() {
	if c.Prefix == "" {
		c.Prefix = "v1/"
	}
	if !strings.HasSuffix(c.Prefix, "/") {
		c.Prefix += "/"
	}
	if c.Type == typeOSS {
		c.Type, c.Provider = TypeS3, ProviderAliyun
		// 旧版界面允许填 "oss-cn-shanghai"，S3 签名要的是 "cn-shanghai"
		c.Region = strings.TrimPrefix(c.Region, "oss-")
		c.Endpoint = aliyunS3Endpoint(c.Endpoint)
	}
	if c.Type == TypeS3 && c.Provider == "" {
		c.Provider = ProviderOther
	}
}

var ErrNotFound = errors.New("not found")

type Object struct {
	Key      string // 不含前缀
	ETag     string
	Size     int64
	Modified time.Time
}

// Storage 是"中转站"。key 都是相对路径（不含配置里的前缀），用 / 分隔。
//
// 注意：S3 兼容服务大多不支持条件写（If-Match），所以"防止两台设备并发覆盖"
// 靠上层（engine 的 putVerified：写完立刻读回校验）实现，而不是这里的接口。
type Storage interface {
	List(ctx context.Context, prefix string) ([]Object, error)
	Get(ctx context.Context, key string) (data []byte, etag string, err error)
	Put(ctx context.Context, key string, data []byte) (etag string, err error)
	// Stat 只取文件信息（etag、大小），不下载内容。不存在返回 ErrNotFound。
	Stat(ctx context.Context, key string) (Object, error)
}

// New 按配置创建存储。c 应已 Normalize（config.Load 和 service 层都会做）。
func New(c Config) (Storage, error) {
	switch c.Type {
	case TypeS3:
		return newS3(c)
	case TypeLocal:
		if c.Path == "" {
			return nil, errors.New(i18n.T("本地存储需要 path"))
		}
		return &localStorage{root: filepath.Join(c.Path, filepath.FromSlash(c.Prefix))}, nil
	}
	return nil, fmt.Errorf(i18n.T("不支持的存储类型: %q"), c.Type)
}
