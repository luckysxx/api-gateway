package main

import (
	"context"
	"log"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/luckysxx/common/logger"
	"github.com/luckysxx/common/metrics"
	"github.com/luckysxx/common/otel"

	"api-gateway/internal/auth"
	"api-gateway/internal/config"
	"api-gateway/internal/middleware"
	"api-gateway/internal/proxy"

	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
)

// ReverseProxyWrapper 封装原生反向代理为 Gin 兼容的 HandlerFunc
func ReverseProxyWrapper(targetUrl string) gin.HandlerFunc {
	// 复用我们在 proxy.go 写的代理引擎
	p := proxy.NewReverseProxy(targetUrl)
	return func(c *gin.Context) {
		// 一旦匹配，立刻用官方纯 C 语言底层的 proxy 接管 HTTP IO
		p.ServeHTTP(c.Writer, c.Request)
	}
}

func main() {
	// 1. 规范化配置
	cfg := config.LoadConfig()

	// 2. 规范化日志
	logApp := logger.NewLogger("api-gateway")
	defer logApp.Sync()

	// 3. 初始化鉴权依赖 (拿配置的 Secret 生成网关的 JWT 管理器)
	jwtManager := auth.NewJWTManager(cfg.JWT.Secret)

	// 设置 Gin 运行模式：Release 模式关闭 GIN 自带的 debug 日志
	// 所有请求日志由我们的 zap 中间件统一管理
	if cfg.AppEnv == "production" || cfg.AppEnv == "prod" {
		gin.SetMode(gin.ReleaseMode)
	} else {
		gin.SetMode(gin.ReleaseMode) // 即使开发环境也用 Release，避免 GIN debug 输出干扰 zap 日志
	}

	r := gin.New()

	// 设置受信任的代理，消除 "You trusted all proxies" 警告
	// 仅信任内网代理（Docker 网络、K8s 集群内网、本机）
	r.SetTrustedProxies([]string{
		"127.0.0.1",
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
	})

	r.GET("/metrics", metrics.GinMetricsHandler())
	r.Use(metrics.GinMetrics())

	// [全局前置拦截器]
	r.Use(otelgin.Middleware("api-gateway"))    // 把每一条来到网关的请求打上 UUID
	r.Use(middleware.GinLogger(logApp))         // 记录响应时长和 UUID
	r.Use(middleware.GinRecovery(logApp, true)) // 崩溃接管防闪退

	// 初始化 OpenTelemetry
	shutdown, err := otel.InitTracer(cfg.OTel.ServiceName, cfg.OTel.JaegerEndpoint)
	if err != nil {
		logApp.Fatal("初始化 OpenTelemetry 失败", zap.Error(err))
	}
	defer shutdown(context.Background())
	// 路由设计：
	api := r.Group("/api/v1")
	{
		// 用户模块代理
		usersGroup := api.Group("/users")
		usersGroup.Use(middleware.JWTAuth(jwtManager, logApp))
		usersGroup.Any("", ReverseProxyWrapper(cfg.Routes.UserPlatform))
		usersGroup.Any("/*any", ReverseProxyWrapper(cfg.Routes.UserPlatform))

		// 笔记模块代理 (go-note 后端)
		// 经过跨端查阅，你的独立微服务 go-note 依然使用统一鉴权体系
		notesGroup := api.Group("")
		notesGroup.Use(middleware.JWTAuth(jwtManager, logApp))
		notesGroup.Any("/me/pastes", ReverseProxyWrapper(cfg.Routes.GoNote))
		notesGroup.Any("/me/pastes/*any", ReverseProxyWrapper(cfg.Routes.GoNote))
		notesGroup.Any("/pastes", ReverseProxyWrapper(cfg.Routes.GoNote))
		notesGroup.Any("/pastes/*any", ReverseProxyWrapper(cfg.Routes.GoNote))
	}

	// 启动网关服务
	logApp.Info("API Gateway 启动成功, 纯 Gin 版本重构完毕",
		zap.String("port", cfg.Server.Port),
	)

	if err := r.Run(":" + cfg.Server.Port); err != nil {
		log.Fatalf("API网关运行异常: %v", err)
	}
}
