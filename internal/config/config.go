package config

import (
	"log"
	"strings"

	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

type Config struct {
	AppEnv string       `mapstructure:"app_env"`
	Server ServerConfig `mapstructure:"server"`
	Routes RoutesConfig `mapstructure:"routes"`
	JWT    JWTConfig    `mapstructure:"jwt"`
	Redis  RedisConfig  `mapstructure:"redis"`
	OTel   OTelConfig   `mapstructure:"otel"`
}

type ServerConfig struct {
	Port string `mapstructure:"port"`
}

type RoutesConfig struct {
	UserPlatform string `mapstructure:"user_platform"`
	GoNote       string `mapstructure:"go_note"`
}

type JWTConfig struct {
	Secret string `mapstructure:"secret"`
}

type RedisConfig struct {
	Addr     string `mapstructure:"addr"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
}

type OTelConfig struct {
	JaegerEndpoint string `mapstructure:"jaeger_endpoint"`
	ServiceName    string `mapstructure:"service_name"`
}

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
