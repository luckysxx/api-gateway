package grpcclient

import (
	"time"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
)

// DefaultDialOptions 返回生产级 gRPC 客户端的标准 DialOption 集合。
//
// 包含：
//   - 明文传输（内网通信，不需要 TLS）
//   - OTel 链路追踪（自动传播 TraceID）
//   - KeepAlive 心跳（防止空闲连接被中间设备关闭）
//   - 熔断器（下游持续故障时快速失败）
//   - 自动重试（仅对可恢复的错误生效）
//
// target 用于标识下游服务，每个 target 有独立的熔断器状态。
func DefaultDialOptions(target string) []grpc.DialOption {
	return []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                10 * time.Second, // 每 10 秒发一次心跳
			Timeout:             3 * time.Second,  // 3 秒没响应视为连接断开
			PermitWithoutStream: true,             // 即使没有活跃 RPC 也保持心跳
		}),
		grpc.WithDefaultServiceConfig(retryPolicy()),
		// 熔断器拦截器 — 在重试之前判断是否需要快速失败
		grpc.WithChainUnaryInterceptor(CircuitBreakerInterceptor(target)),
	}
}

// retryPolicy 返回 gRPC 内置重试策略的 JSON 配置。
//
// 策略说明：
//   - 最多重试 3 次（含首次调用共 4 次）
//   - 初始退避 100ms，最大退避 1s，退避因子 2.0（指数退避）
//   - 仅在 UNAVAILABLE 和 DEADLINE_EXCEEDED 时重试
//   - gRPC 自动判断幂等性：只有被标记为 WaitForReady 或服务端返回可重试状态码时才重试
func retryPolicy() string {
	return `{
		"methodConfig": [{
			"name": [{"service": ""}],
			"timeout": "3s",
			"retryPolicy": {
				"maxAttempts": 4,
				"initialBackoff": "0.1s",
				"maxBackoff": "1s",
				"backoffMultiplier": 2.0,
				"retryableStatusCodes": ["UNAVAILABLE", "DEADLINE_EXCEEDED"]
			}
		}]
	}`
}
