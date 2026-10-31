package storage

// S3 兼容对象存储实现（minio-go）。一套代码连阿里云 OSS、腾讯云 COS、Cloudflare R2、AWS S3、MinIO 等。
//
// 兼容性要点（都是踩过坑的服务商限制）：
//   - 阿里云 OSS 不支持 aws-chunked 分块签名：上传一律用 UNSIGNED-PAYLOAD + Content-MD5，不用流式签名
//   - 阿里云 OSS、腾讯云 COS 要求虚拟主机风格（bucket.endpoint）；MinIO 等自建服务通常要路径风格（endpoint/bucket）
//   - 不开 TrailingHeaders：带尾部校验和的上传同样是 aws-chunked

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"github.com/ALFO111111/agent-sync/internal/i18n"
)

// ResolvedEndpoint 实际连接的地址：填了 Endpoint 用它，否则按服务商+地域生成（R2、MinIO、其他必须手填）。
func (c Config) ResolvedEndpoint() string {
	if e := strings.TrimSpace(c.Endpoint); e != "" {
		return e
	}
	r := strings.TrimSpace(c.Region)
	if r == "" {
		return ""
	}
	switch c.Provider {
	case ProviderAliyun:
		return "https://s3.oss-" + r + ".aliyuncs.com"
	case ProviderTencent:
		return "https://cos." + r + ".myqcloud.com"
	case ProviderAWS:
		return "https://s3." + r + ".amazonaws.com"
	}
	return ""
}

// signingRegion 签名用的地域。没填时：R2 固定 auto；阿里云从 endpoint 里认；其他用 us-east-1（MinIO 默认）。
// 一定要给个值：留空的话 minio-go 会先多发一次 GetBucketLocation，很多服务商、最小权限的子账号都不支持。
func (c Config) signingRegion(endpointHost string) string {
	if r := strings.TrimSpace(c.Region); r != "" {
		return r
	}
	if c.Provider == ProviderR2 {
		return "auto"
	}
	if m := aliyunHostRe.FindStringSubmatch(endpointHost); m != nil {
		return m[1]
	}
	return "us-east-1"
}

// aliyunHostRe 阿里云 OSS 的 endpoint：[s3.]oss-<地域>[-internal].aliyuncs.com
var aliyunHostRe = regexp.MustCompile(`^(?:s3\.)?oss-([a-z0-9-]+?)(?:-internal)?\.aliyuncs\.com$`)

// aliyunS3Endpoint 旧版 OSS 原生 endpoint（oss-cn-shanghai.aliyuncs.com）→ S3 兼容 endpoint（s3.oss-cn-shanghai.aliyuncs.com）。
// 空串、已经是 S3 写法、或者认不出来（比如自定义域名）的原样返回。
func aliyunS3Endpoint(e string) string {
	e = strings.TrimSpace(e)
	if e == "" {
		return ""
	}
	scheme, host := "https://", e
	if i := strings.Index(e, "://"); i >= 0 {
		scheme, host = e[:i+3], e[i+3:]
	}
	host = strings.TrimSuffix(host, "/")
	if strings.HasPrefix(host, "oss-") && aliyunHostRe.MatchString(host) {
		return scheme + "s3." + host
	}
	return e
}

func newS3(c Config) (Storage, error) {
	if c.Bucket == "" || c.AccessKeyID == "" || c.AccessKeySecret == "" {
		return nil, errors.New(i18n.T("S3 配置不完整（需要 Bucket、AccessKeyId、AccessKeySecret）"))
	}
	raw := c.ResolvedEndpoint()
	if raw == "" {
		return nil, errors.New(i18n.T("S3 配置不完整：请填写 Endpoint（阿里云、腾讯云、AWS 也可以只填地域）"))
	}
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return nil, errors.New(i18n.T("Endpoint 格式不对，应该像 https://s3.example.com"))
	}
	lookup := minio.BucketLookupDNS
	if c.PathStyle {
		lookup = minio.BucketLookupPath
	}
	client, err := minio.New(u.Host, &minio.Options{
		Creds:        credentials.NewStaticV4(c.AccessKeyID, c.AccessKeySecret, ""),
		Secure:       u.Scheme == "https",
		Region:       c.signingRegion(u.Hostname()),
		BucketLookup: lookup,
		Transport:    s3Transport(),
	})
	if err != nil {
		return nil, err
	}
	return &s3Storage{client: client, bucket: c.Bucket, prefix: c.Prefix}, nil
}

// s3Transport 连接超时 10 秒（和旧版 OSS 一致），读写不设总超时：大文件慢慢传，靠调用方的 ctx 控制。
func s3Transport() http.RoundTripper {
	t := http.DefaultTransport.(*http.Transport).Clone()
	t.ResponseHeaderTimeout = 60 * time.Second
	t.TLSHandshakeTimeout = 10 * time.Second
	return t
}

type s3Storage struct {
	client *minio.Client
	bucket string
	prefix string
}

func (s *s3Storage) List(ctx context.Context, prefix string) ([]Object, error) {
	var out []Object
	for o := range s.client.ListObjects(ctx, s.bucket, minio.ListObjectsOptions{Prefix: s.prefix + prefix, Recursive: true}) {
		if o.Err != nil {
			return nil, o.Err
		}
		out = append(out, Object{
			Key:      strings.TrimPrefix(o.Key, s.prefix),
			ETag:     normETag(o.ETag),
			Size:     o.Size,
			Modified: o.LastModified,
		})
	}
	return out, nil
}

func (s *s3Storage) Get(ctx context.Context, key string) ([]byte, string, error) {
	obj, err := s.client.GetObject(ctx, s.bucket, s.prefix+key, minio.GetObjectOptions{})
	if err != nil {
		return nil, "", s3Err(err)
	}
	defer obj.Close()
	data, err := io.ReadAll(obj) // GetObject 是懒请求：真正的 404 在第一次读时才出现
	if err != nil {
		return nil, "", s3Err(err)
	}
	etag := ""
	if info, err := obj.Stat(); err == nil { // 读过之后 Stat 用的是同一个响应，不会再发请求
		etag = normETag(info.ETag)
	}
	return data, etag, nil
}

func (s *s3Storage) Put(ctx context.Context, key string, data []byte) (string, error) {
	info, err := s.client.PutObject(ctx, s.bucket, s.prefix+key, bytes.NewReader(data), int64(len(data)), minio.PutObjectOptions{
		ContentType:          "application/octet-stream",
		SendContentMd5:       true, // 服务端校验完整性，代替 sha256（见文件顶部：不能用流式签名）
		DisableContentSha256: true, // http 地址也不走 aws-chunked 流式签名
	})
	if err != nil {
		return "", s3Err(err)
	}
	return normETag(info.ETag), nil
}

func s3Err(err error) error {
	if r := minio.ToErrorResponse(err); r.Code == "NoSuchKey" || r.StatusCode == http.StatusNotFound && r.Code != "NoSuchBucket" {
		return ErrNotFound
	}
	return err
}

// normETag 统一成去引号的大写：和旧版 OSS SDK 存进 state.json 的格式一致，
// 换实现后不会因为大小写不同把所有文件误判成"云端变了"。
func normETag(e string) string { return strings.ToUpper(strings.Trim(e, `"`)) }

func (s *s3Storage) Stat(ctx context.Context, key string) (Object, error) {
	info, err := s.client.StatObject(ctx, s.bucket, s.prefix+key, minio.StatObjectOptions{})
	if err != nil {
		return Object{}, s3Err(err)
	}
	return Object{Key: key, ETag: normETag(info.ETag), Size: info.Size, Modified: info.LastModified}, nil
}
