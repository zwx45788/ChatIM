// pkg/config/config.go
package config

import (
	"log"

	"github.com/spf13/viper"
)

type Config struct {
	Server   ServerConfig   `mapstructure:"server"`
	Database DatabaseConfig `mapstructure:"database"`
	JWT      JWTConfig      `mapstructure:"jwt"`
}

type ServerConfig struct {
	APIPort         string `mapstructure:"api_port"`
	UserGRPCPort    string `mapstructure:"user_grpc_port"`
	MessageGRPCPort string `mapstructure:"message_grpc_port"`
}

type DatabaseConfig struct {
	MySQL MySQLConfig `mapstructure:"mysql"`
	Redis RedisConfig `mapstructure:"redis"`
}

type MySQLConfig struct {
	DSN string `mapstructure:"dsn"`
}

type RedisConfig struct {
	Addr     string `mapstructure:"addr"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
}

type JWTConfig struct {
	Secret string `mapstructure:"secret"`
}

// LoadConfig 加载配置文件
func LoadConfig() (*Config, error) {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	// viper.AddConfigPath(".")
	// viper.AddConfigPath("./config")
	viper.SetConfigFile("D:/git-demo/ChatIM/pkg/config/config.yaml")
	// 支持环境变量覆盖配置，例如 CHATIM_SERVER_API_PORT
	viper.AutomaticEnv()
	viper.SetEnvPrefix("CHATIM") // 环境变量前缀

	if err := viper.ReadInConfig(); err != nil {
		log.Fatalf("Error reading config file, %s", err)
	}

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		log.Fatalf("Error unmarshaling config, %s", err)
	}

	return &config, nil
}
