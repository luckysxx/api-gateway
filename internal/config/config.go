package config

import (
	"log"
	"strings"

	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

// Config 定义了 API Gateway 的完整配置结构。
type Config struct {
	AppEnv string       `mapstructure:"app_env"`
	Server ServerConfig `mapstructure:"server"`
	Routes RoutesConfig `mapstructure:"routes"`
	JWT    JWTConfig    `mapstructure:"jwt"`
	Redis  RedisConfig  `mapstructure:"redis"`
	OTel   OTelConfig   `mapstructure:"otel"`
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
	GoChat           string `mapstructure:"go_chat"`
}

// JWTConfig 定义了网关验签使用的 JWT 配置。
type JWTConfig struct {
	Secret string `mapstructure:"secret"`
}

// RedisConfig 定义了网关 Redis 连接配置。
type RedisConfig struct {
	Addr     string `mapstructure:"addr"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
}

// OTelConfig 定义了网关链路追踪配置。
type OTelConfig struct {
	JaegerEndpoint string `mapstructure:"jaeger_endpoint"`
	ServiceName    string `mapstructure:"service_name"`
}

// LoadConfig 从配置文件和环境变量中加载网关配置。
func LoadConfig() *Config {
	_ = godotenv.Load()

	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("./configs")
	viper.AddConfigPath(".")

	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		log.Printf("Warning: No config.yaml found, relying entirely on ENV variables: %v", err)
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		log.Fatalf("Failed to unmarshal config: %v", err)
	}
	return &cfg
}
