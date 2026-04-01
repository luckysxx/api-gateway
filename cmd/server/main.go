package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/luckysxx/common/health"
	"github.com/luckysxx/common/logger"
	"github.com/luckysxx/common/metrics"
	"github.com/luckysxx/common/otel"
	"github.com/luckysxx/common/ratelimiter"
	"github.com/luckysxx/common/redis"

	"api-gateway/internal/auth"
	"api-gateway/internal/config"
	"api-gateway/internal/grpcclient"
	"api-gateway/internal/handler"
	"api-gateway/internal/middleware"
	"api-gateway/internal/middleware/ratelimit"
	"api-gateway/internal/proxy"
	"api-gateway/internal/restclient"

	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
)

func main() {
	// 规范化配置
	cfg := config.LoadConfig()

	// 规范化日志
	log := logger.NewLogger("api-gateway")
	defer log.Sync()

	// 初始化 Redis
	redisClient := redis.Init(redis.Config{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	}, log)
	defer redisClient.Close()

	// 初始化 gRPC 客户端（使用纯 host:port 格式的 gRPC 地址）
	authClient, err := grpcclient.NewAuthClient(cfg.Routes.UserPlatformGRPC)
	if err != nil {
		log.Fatal("初始化 Auth gRPC 客户端失败", zap.Error(err))
	}
	userClient, err := grpcclient.NewUserClient(cfg.Routes.UserPlatformGRPC)
	if err != nil {
		log.Fatal("初始化 User gRPC 客户端失败", zap.Error(err))
	}
	// 初始化 REST 客户端（使用 http:// 格式的 HTTP 地址）
	noteClientRest := restclient.NewNoteClient(cfg.Routes.GoNote, log)

	// [双轨制] 初始化 gRPC 客户端
	noteClientGrpc, err := grpcclient.NewNoteClient(cfg.Routes.GoNoteGRPC)
	if err != nil {
		log.Error("初始化 Note gRPC 客户端失败", zap.Error(err))
	}

	chatProxy := proxy.NewReverseProxy(cfg.Routes.GoChat)

	// ====== [双轨制协议开关] ======
	// 控制从网关流向 note-service 的请求是走 gRPC 还是 HTTP REST
	useGrpcForNotes := true

	// 初始化 handler
	dashboardHandler := handler.NewDashboardHandler(userClient, noteClientRest, log) // Dashbaord 当前未重构暂走REST
	authHandler := handler.NewAuthHandler(authClient, log)
	userHandler := handler.NewUserHandler(userClient, log)
	
	noteHandlerRestInstance := handler.NewNoteHandler(noteClientRest, log)
	noteHandlerGrpcInstance := handler.NewNoteHandlerGrpc(noteClientGrpc, log)

	chatHandler := handler.NewChatHandler(chatProxy)

	// 初始化限流器
	BBRLimiter := ratelimiter.NewBBRLimiter(100, 10*time.Second, 80)
	IPLimiter := ratelimiter.NewSlidingWindowLimiter(redisClient, log)
	RouteLimiter := ratelimiter.NewTokenBucketLimiter(redisClient, log)
	UserLimiter := ratelimiter.NewSlidingWindowLimiter(redisClient, log)

	// 初始化鉴权依赖 (拿配置的 Secret 生成网关的 JWT 管理器)
	jwtManager := auth.NewJWTManager(cfg.JWT.Secret)

	// 设置 Gin 运行模式：Release 模式关闭 GIN 自带的 debug 日志
	// 所有请求日志由我们的 zap 中间件统一管理
	if cfg.AppEnv == "production" || cfg.AppEnv == "prod" {
		gin.SetMode(gin.ReleaseMode)
	} else {
		gin.SetMode(gin.ReleaseMode) // 即使开发环境也用 Release，避免 GIN debug 输出干扰 zap 日志
	}

	// [Bug 5 修复] 初始化 OpenTelemetry — 必须在注册 otelgin 中间件之前
	shutdown, err := otel.InitTracer(cfg.OTel.ServiceName, cfg.OTel.JaegerEndpoint)
	if err != nil {
		log.Fatal("初始化 OpenTelemetry 失败", zap.Error(err))
	}
	defer shutdown(context.Background())

	r := gin.New()

	// 设置受信任的代理，消除 "You trusted all proxies" 警告
	// 仅信任内网代理（Docker 网络、K8s 集群内网、本机）
	r.SetTrustedProxies([]string{
		"127.0.0.1",
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
	})

	// [Bug 3 修复] 全局注入 logger 到每个请求的 context，确保所有路径都能记录日志
	r.Use(func(c *gin.Context) {
		c.Set("logger", log)
		c.Next()
	})

	// 健康检查（注册在所有中间件之前，避免被限流/鉴权拦截）
	healthChecker := health.NewChecker()
	healthChecker.AddCheck("redis", func(ctx context.Context) error {
		return redisClient.Ping(ctx).Err()
	})
	healthChecker.AddCheck("go-note", func(ctx context.Context) error {
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, cfg.Routes.GoNote+"/readyz", nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return err
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("go-note readiness returned status %d", resp.StatusCode)
		}
		return nil
	})
	healthChecker.AddCheck("go-chat", func(ctx context.Context) error {
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, cfg.Routes.GoChat+"/readyz", nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return err
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("go-chat readiness returned status %d", resp.StatusCode)
		}
		return nil
	})
	healthChecker.Register(r)

	r.GET("/metrics", metrics.GinMetricsHandler())
	r.Use(metrics.GinMetrics())

	// [CORS 防御层] — 从配置中读取白名单
	if len(cfg.Server.CorsOrigins) > 0 {
		r.Use(cors.New(cors.Config{
			AllowOrigins:     cfg.Server.CorsOrigins,
			AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
			AllowHeaders:     []string{"Origin", "Content-Type", "Authorization", "X-Request-Id"},
			ExposeHeaders:    []string{"Content-Length", "X-Request-Id"},
			AllowCredentials: true,
			MaxAge:           12 * time.Hour,
		}))
	}

	// [全局前置拦截器]
	r.Use(otelgin.Middleware("api-gateway")) // 把每一条来到网关的请求打上 TraceID
	r.Use(logger.GinLogger(log))             // 记录响应时长和 TraceID（来自 common/logger）
	r.Use(logger.GinRecovery(log, true))     // 崩溃接管防闪退
	// 路由设计：
	api := r.Group("/api/v1")
	// Layer 1: IP 限流 (滑动窗口) - 所有请求都过
	api.Use(ratelimit.IPrateLimit(IPLimiter, 50, time.Second, log))
	// Layer 2: BBR 自适应限流 - 所有请求都过
	api.Use(ratelimit.BBRMiddleware(BBRLimiter, log))
	// 公共接口（无需 JWT 鉴权）
	api.POST("/users/register", userHandler.Register)
	api.POST("/users/login", authHandler.Login)
	api.POST("/users/refresh", authHandler.RefreshToken)
	if useGrpcForNotes {
		api.GET("/notes/public/snippets/:id", noteHandlerGrpcInstance.GetPublic)
	} else {
		api.GET("/notes/public/snippets/:id", noteHandlerRestInstance.GetPublic)
	}

	{
		// 用户模块路由组（需 JWT 鉴权）
		usersGroup := api.Group("/users")
		// Layer 3: 路由限流 (令牌桶) - 单个服务级别
		usersGroup.Use(ratelimit.RouteRateLimit(RouteLimiter, 200, 10*time.Second, log))
		usersGroup.Use(middleware.JWTAuth(jwtManager, log))
		// Layer 4: 用户限流 (滑动窗口) - 登录用户过
		usersGroup.Use(ratelimit.UserRateLimit(UserLimiter, 20, time.Second, log))
		usersGroup.GET("/dashboard", dashboardHandler.GetDashboard)
		usersGroup.GET("/me/profile", userHandler.GetProfile)
		usersGroup.PUT("/me/profile", userHandler.UpdateProfile)
		usersGroup.POST("/logout", authHandler.Logout)
	}
	{
		// 笔记模块路由组（需 JWT 鉴权）
		notesGroup := api.Group("/notes")
		notesGroup.Use(ratelimit.RouteRateLimit(RouteLimiter, 200, 10*time.Second, log))
		notesGroup.Use(middleware.JWTAuth(jwtManager, log))
		notesGroup.Use(ratelimit.UserRateLimit(UserLimiter, 20, time.Second, log))

		if useGrpcForNotes {
			notesGroup.GET("/me/snippets", noteHandlerGrpcInstance.ListMine)
			notesGroup.POST("/snippets", noteHandlerGrpcInstance.Create)
			notesGroup.GET("/snippets/:id", noteHandlerGrpcInstance.Get)
			notesGroup.PUT("/snippets/:id", noteHandlerGrpcInstance.Update)

			// 片段扩展
			notesGroup.DELETE("/snippets/:id", noteHandlerGrpcInstance.Delete)
			notesGroup.GET("/snippets/search", noteHandlerGrpcInstance.Search)
			notesGroup.POST("/snippets/:id/favorite", noteHandlerGrpcInstance.Favorite)
			notesGroup.DELETE("/snippets/:id/favorite", noteHandlerGrpcInstance.Unfavorite)
			notesGroup.POST("/snippets/from-template", noteHandlerGrpcInstance.CreateFromTemplate)

			// 工作区列表
			notesGroup.GET("/me/snippets/recent", noteHandlerGrpcInstance.ListRecent)
			notesGroup.GET("/me/snippets/shared", noteHandlerGrpcInstance.ListShared)
			notesGroup.GET("/me/snippets/favorites", noteHandlerGrpcInstance.ListFavorites)

			// 分组与标签
			notesGroup.GET("/groups", noteHandlerGrpcInstance.GetGroups)
			notesGroup.POST("/groups", noteHandlerGrpcInstance.CreateGroup)
			notesGroup.PUT("/groups/:id", noteHandlerGrpcInstance.UpdateGroup)
			notesGroup.DELETE("/groups/:id", noteHandlerGrpcInstance.DeleteGroup)

			notesGroup.GET("/tags", noteHandlerGrpcInstance.GetTags)
			notesGroup.POST("/tags", noteHandlerGrpcInstance.CreateTag)
			notesGroup.DELETE("/tags/:id", noteHandlerGrpcInstance.DeleteTag)

			// 模板与上传
			notesGroup.GET("/templates", noteHandlerGrpcInstance.GetTemplates)
			notesGroup.GET("/templates/:id", noteHandlerGrpcInstance.GetTemplate)
			notesGroup.POST("/uploads", noteHandlerGrpcInstance.Upload)
		} else {
			notesGroup.GET("/me/snippets", noteHandlerRestInstance.ListMine)
			notesGroup.POST("/snippets", noteHandlerRestInstance.Create)
			notesGroup.GET("/snippets/:id", noteHandlerRestInstance.Get)
			notesGroup.PUT("/snippets/:id", noteHandlerRestInstance.Update)

			// 片段扩展
			notesGroup.DELETE("/snippets/:id", noteHandlerRestInstance.Delete)
			notesGroup.GET("/snippets/search", noteHandlerRestInstance.Search)
			notesGroup.POST("/snippets/:id/favorite", noteHandlerRestInstance.Favorite)
			notesGroup.DELETE("/snippets/:id/favorite", noteHandlerRestInstance.Unfavorite)
			notesGroup.POST("/snippets/from-template", noteHandlerRestInstance.CreateFromTemplate)

			// 工作区列表
			notesGroup.GET("/me/snippets/recent", noteHandlerRestInstance.ListRecent)
			notesGroup.GET("/me/snippets/shared", noteHandlerRestInstance.ListShared)
			notesGroup.GET("/me/snippets/favorites", noteHandlerRestInstance.ListFavorites)

			// 分组与标签
			notesGroup.GET("/groups", noteHandlerRestInstance.GetGroups)
			notesGroup.POST("/groups", noteHandlerRestInstance.CreateGroup)
			notesGroup.PUT("/groups/:id", noteHandlerRestInstance.UpdateGroup)
			notesGroup.DELETE("/groups/:id", noteHandlerRestInstance.DeleteGroup)

			notesGroup.GET("/tags", noteHandlerRestInstance.GetTags)
			notesGroup.POST("/tags", noteHandlerRestInstance.CreateTag)
			notesGroup.DELETE("/tags/:id", noteHandlerRestInstance.DeleteTag)

			// 模板与上传
			notesGroup.GET("/templates", noteHandlerRestInstance.GetTemplates)
			notesGroup.GET("/templates/:id", noteHandlerRestInstance.GetTemplate)
			notesGroup.POST("/uploads", noteHandlerRestInstance.Upload)
		}
	}
	{
		chatGroup := api.Group("/chat")
		chatGroup.Use(ratelimit.RouteRateLimit(RouteLimiter, 200, 10*time.Second, log))
		chatGroup.Use(middleware.JWTAuth(jwtManager, log))
		chatGroup.Use(ratelimit.UserRateLimit(UserLimiter, 30, time.Second, log))
		chatGroup.Any("/*path", chatHandler.Proxy)
	}

	// 启动网关服务
	srv := &http.Server{
		Addr:    ":" + cfg.Server.Port,
		Handler: r,
	}

	go func() {
		log.Info("API Gateway 启动成功",
			zap.String("port", cfg.Server.Port),
		)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("API网关运行异常: %v", zap.Error(err))
		}
	}()

	// 优雅停机,拦截系统的停机信号（如 Ctrl+C）
	quit := make(chan os.Signal, 1)
	// [Bug 4 修复] 同时监听 SIGINT(Ctrl+C) 和 SIGTERM(Docker/K8s 停止信号)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit
	log.Info("API Gateway 正在关闭...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal("API网关强制关闭", zap.Error(err))
	}
	log.Info("API Gateway 已关闭")
}
