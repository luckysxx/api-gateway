package config

import (
	"github.com/luckysxx/common/conf"
	commonOtel "github.com/luckysxx/common/otel"
	commonRedis "github.com/luckysxx/common/redis"
)

// Config 定义了 API Gateway 的完整配置结构。
type Config struct {
	AppEnv string            `mapstructure:"app_env"`
	Server ServerConfig      `mapstructure:"server"`
	Routes RoutesConfig      `mapstructure:"routes"`
	JWT    JWTConfig         `mapstructure:"jwt"`
	Redis  commonRedis.Config `mapstructure:"redis"`
	OTel   commonOtel.Config  `mapstructure:"otel"`
}

// ServerConfig 定义了网关监听和跨域配置。
type ServerConfig struct {
	Port        string   `mapstructure:"port"`
	CorsOrigins []string `mapstructure:"cors_origins"`
}

// RoutesConfig 定义了网关访问下游服务的地址配置。
type RoutesConfig struct {
	UserPlatformHTTP string `mapstructure:"user_platform_http"`
	UserPlatformGRPC string `mapstructure:"user_platform_grpc"`
	GoNote           string `mapstructure:"go_note"`
	GoNoteGRPC       string `mapstructure:"go_note_grpc"`
	GoChat           string `mapstructure:"go_chat"`
}

// JWTConfig 定义了网关验签使用的 JWT 配置。
type JWTConfig struct {
	Secret string `mapstructure:"secret"`
}

// LoadConfig 从配置文件和环境变量中加载网关配置。
func LoadConfig() *Config {
	var cfg Config
	conf.Load(&cfg)
	return &cfg
}

