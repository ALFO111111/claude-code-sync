package update

import (
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// httpClient 访问 GitHub 用的客户端。国内直连 GitHub 经常连不上：除了 HTTPS_PROXY 这类环境变量，
// 还认系统代理（Windows 的"设置 → 网络 → 代理"、macOS 的网络代理，Clash 之类的工具开"系统代理"时就写在这里）。
func httpClient() *http.Client {
	t := http.DefaultTransport.(*http.Transport).Clone()
	t.Proxy = proxyFor
	t.ResponseHeaderTimeout = 30 * time.Second
	t.TLSHandshakeTimeout = 15 * time.Second
	return &http.Client{Transport: t}
}

func proxyFor(r *http.Request) (*url.URL, error) {
	if u, err := http.ProxyFromEnvironment(r); u != nil || err != nil {
		return u, err
	}
	if h := r.URL.Hostname(); h == "localhost" || net.ParseIP(h) != nil && net.ParseIP(h).IsLoopback() {
		return nil, nil // 本机地址不走代理（和 ProxyFromEnvironment 的规则一样）
	}
	return parseProxy(systemProxy())
}

// parseProxy "127.0.0.1:7890" / "http://127.0.0.1:7890" → URL；空串返回 nil（直连）。
func parseProxy(s string) (*url.URL, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	if !strings.Contains(s, "://") {
		s = "http://" + s
	}
	return url.Parse(s)
}

// pickWinProxy Windows 的 ProxyServer 可能是 "host:port"，也可能按协议分开写 "http=h:p;https=h:p;socks=h:p"。
func pickWinProxy(server string) string {
	if !strings.Contains(server, "=") {
		return server
	}
	byScheme := map[string]string{}
	for _, part := range strings.Split(server, ";") {
		if k, v, ok := strings.Cut(strings.TrimSpace(part), "="); ok {
			byScheme[strings.ToLower(k)] = v
		}
	}
	for _, k := range []string{"https", "http"} {
		if v := byScheme[k]; v != "" {
			return v
		}
	}
	if v := byScheme["socks"]; v != "" {
		return "socks5://" + v
	}
	return ""
}

// pickMacProxy 解析 `scutil --proxy` 的输出，先用 HTTPS 代理，没有再用 HTTP 代理。
func pickMacProxy(out string) string {
	kv := map[string]string{}
	for _, line := range strings.Split(out, "\n") {
		if k, v, ok := strings.Cut(line, " : "); ok {
			kv[strings.TrimSpace(k)] = strings.TrimSpace(v)
		}
	}
	for _, p := range []string{"HTTPS", "HTTP"} {
		if kv[p+"Enable"] == "1" && kv[p+"Proxy"] != "" {
			host := kv[p+"Proxy"]
			if port := kv[p+"Port"]; port != "" {
				host += ":" + port
			}
			return host
		}
	}
	return ""
}
