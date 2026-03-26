package proxy

import (
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

// NewReverseProxy 封装官方库，创建一个带极致性能连接池的反向代理引擎
func NewReverseProxy(targetHost string) *httputil.ReverseProxy {
	// 解析目标下游服务的地址
	targetURL, err := url.Parse(targetHost)
	if err != nil {
		log.Fatalf("解析目标地址失败: %v", err)
	}

	// 这就是原生反向代理引擎
	proxy := httputil.NewSingleHostReverseProxy(targetURL)

	// 调优网关跟下游微服务之间的 HTTP 连接池 (Connection Pool)
	proxy.Transport = &http.Transport{
		MaxIdleConns:        1000,             // 整个连接池最大空闲连接数
		MaxIdleConnsPerHost: 200,              // 每个下游服务 Host 的最大空闲长连接数
		IdleConnTimeout:     90 * time.Second, // 长连接空闲多久不断开可以复用
		DisableKeepAlives:   false,            // 绝不允许每次请求都重新排队搞 TCP 三次握手
		ForceAttemptHTTP2:   true,             // 强制尝试开启 HTTP/2 多路复用机制
	}

	defaultDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		defaultDirector(req)
		otel.GetTextMapPropagator().Inject(req.Context(), propagation.HeaderCarrier(req.Header))
	}
	return proxy
}
