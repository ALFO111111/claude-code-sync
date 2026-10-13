package storage

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/johannesboyne/gofakes3"
	"github.com/johannesboyne/gofakes3/backend/s3mem"
)

func TestNormalizeMigratesOldOSS(t *testing.T) {
	cases := []struct{ region, endpoint, wantRegion, wantEndpoint string }{
		{"cn-shanghai", "", "cn-shanghai", "https://s3.oss-cn-shanghai.aliyuncs.com"},
		{"oss-cn-shanghai", "", "cn-shanghai", "https://s3.oss-cn-shanghai.aliyuncs.com"},
		{"cn-shanghai", "oss-cn-shanghai.aliyuncs.com", "cn-shanghai", "https://s3.oss-cn-shanghai.aliyuncs.com"},
		{"cn-hangzhou", "https://oss-cn-hangzhou-internal.aliyuncs.com/", "cn-hangzhou", "https://s3.oss-cn-hangzhou-internal.aliyuncs.com"},
		{"cn-hangzhou", "https://files.example.com", "cn-hangzhou", "https://files.example.com"}, // 自定义域名不动
	}
	for _, c := range cases {
		cfg := Config{Type: "oss", Region: c.region, Endpoint: c.endpoint, Bucket: "b"}
		cfg.Normalize()
		if cfg.Type != TypeS3 || cfg.Provider != ProviderAliyun || cfg.Region != c.wantRegion || cfg.ResolvedEndpoint() != c.wantEndpoint {
			t.Errorf("%+v → %+v，endpoint %s", c, cfg, cfg.ResolvedEndpoint())
		}
		before := cfg
		cfg.Normalize()
		if cfg != before {
			t.Errorf("Normalize 不是幂等的: %+v → %+v", before, cfg)
		}
	}
}

func TestResolvedEndpointAndRegion(t *testing.T) {
	cases := []struct {
		c            Config
		endpoint     string
		signedRegion string
	}{
		{Config{Provider: ProviderAliyun, Region: "cn-beijing"}, "https://s3.oss-cn-beijing.aliyuncs.com", "cn-beijing"},
		{Config{Provider: ProviderAliyun, Endpoint: "https://s3.oss-cn-shenzhen.aliyuncs.com"}, "https://s3.oss-cn-shenzhen.aliyuncs.com", "cn-shenzhen"},
		{Config{Provider: ProviderTencent, Region: "ap-shanghai"}, "https://cos.ap-shanghai.myqcloud.com", "ap-shanghai"},
		{Config{Provider: ProviderAWS, Region: "us-west-2"}, "https://s3.us-west-2.amazonaws.com", "us-west-2"},
		{Config{Provider: ProviderR2, Endpoint: "https://abc.r2.cloudflarestorage.com"}, "https://abc.r2.cloudflarestorage.com", "auto"},
		{Config{Provider: ProviderMinIO, Endpoint: "http://192.168.1.2:9000"}, "http://192.168.1.2:9000", "us-east-1"},
		{Config{Provider: ProviderR2}, "", "auto"}, // R2 必须手填 endpoint
	}
	for _, c := range cases {
		if got := c.c.ResolvedEndpoint(); got != c.endpoint {
			t.Errorf("%+v endpoint = %q，期望 %q", c.c, got, c.endpoint)
		}
		host := strings.TrimPrefix(strings.TrimPrefix(c.endpoint, "https://"), "http://")
		if got := c.c.signingRegion(host); got != c.signedRegion {
			t.Errorf("%+v 签名地域 = %q，期望 %q", c.c, got, c.signedRegion)
		}
	}
}

func TestNewS3Validates(t *testing.T) {
	for _, c := range []Config{
		{Type: TypeS3, Provider: ProviderAliyun, Region: "cn-shanghai", AccessKeyID: "a", AccessKeySecret: "s"},            // 没 bucket
		{Type: TypeS3, Provider: ProviderR2, Bucket: "b", AccessKeyID: "a", AccessKeySecret: "s"},                          // R2 没 endpoint
		{Type: TypeS3, Provider: ProviderOther, Endpoint: "https://", Bucket: "b", AccessKeyID: "a", AccessKeySecret: "s"}, // endpoint 格式不对
	} {
		if _, err := New(c); err == nil {
			t.Errorf("%+v 应该报配置错误", c)
		}
	}
}

// TestS3RoundTrip 对着内存里的假 S3 服务跑一遍读写，并检查上传请求的签名方式：
// 必须是 UNSIGNED-PAYLOAD + Content-MD5，不能是阿里云 OSS 不支持的 aws-chunked 分块签名。
func TestS3RoundTrip(t *testing.T) {
	backend := s3mem.New()
	if err := backend.CreateBucket("sync-bucket"); err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	var puts []http.Header
	fake := gofakes3.New(backend).Server()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut {
			mu.Lock()
			puts = append(puts, r.Header.Clone())
			mu.Unlock()
		}
		fake.ServeHTTP(w, r)
	}))
	defer srv.Close()

	cfg := Config{Type: TypeS3, Provider: ProviderMinIO, Endpoint: srv.URL, PathStyle: true,
		Bucket: "sync-bucket", AccessKeyID: "ak", AccessKeySecret: "sk"}
	cfg.Normalize()
	s, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	if _, _, err := s.Get(ctx, "keycheck"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("不存在的对象应返回 ErrNotFound，得到 %v", err)
	}
	etag, err := s.Put(ctx, "files/abc/x.jsonl", []byte("hello"))
	if err != nil {
		t.Fatal(err)
	}
	if etag == "" || etag != strings.ToUpper(etag) || strings.Contains(etag, `"`) {
		t.Errorf("etag 应是去引号的大写: %q", etag)
	}
	data, _, err := s.Get(ctx, "files/abc/x.jsonl")
	if err != nil || string(data) != "hello" {
		t.Fatalf("读回 %q, %v", data, err)
	}
	objs, err := s.List(ctx, "files/")
	if err != nil {
		t.Fatal(err)
	}
	if len(objs) != 1 || objs[0].Key != "files/abc/x.jsonl" || objs[0].ETag != etag || objs[0].Size != 5 {
		t.Errorf("List = %+v，期望一个 key=files/abc/x.jsonl etag=%s", objs, etag)
	}
	if objs, _ := s.List(ctx, "devices/"); len(objs) != 0 {
		t.Errorf("前缀过滤失效: %+v", objs)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(puts) != 1 {
		t.Fatalf("应该只有 1 次 PUT，实际 %d", len(puts))
	}
	h := puts[0]
	if got := h.Get("X-Amz-Content-Sha256"); got != "UNSIGNED-PAYLOAD" {
		t.Errorf("X-Amz-Content-Sha256 = %q，期望 UNSIGNED-PAYLOAD（阿里云 OSS 不支持流式签名）", got)
	}
	if h.Get("Content-Md5") == "" {
		t.Error("上传应带 Content-MD5")
	}
	if strings.Contains(h.Get("Content-Encoding"), "aws-chunked") || h.Get("X-Amz-Trailer") != "" {
		t.Errorf("不能用 aws-chunked / 尾部校验和: %v", h)
	}
}
