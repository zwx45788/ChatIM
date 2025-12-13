// pkg/config/config.go
package config

import (
	"log"
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	Server   ServerConfig   `mapstructure:"server"`
	Database DatabaseConfig `mapstructure:"database"`
	JWT      JWTConfig      `mapstructure:"jwt"`
}

type ServerConfig struct {
	APIPort            string `mapstructure:"api_port"`
	UserGRPCPort       string `mapstructure:"user_grpc_port"`
	MessageGRPCPort    string `mapstructure:"message_grpc_port"`
	GroupGRPCPort      string `mapstructure:"group_grpc_port"`
	FriendshipGRPCPort string `mapstructure:"friendship_grpc_port"`
	UserGRPCAddr       string `mapstructure:"user_grpc_addr"`       // 新增：User Service 地址（用于 API Gateway 连接）
	MessageGRPCAddr    string `mapstructure:"message_grpc_addr"`    // 新增：Message Service 地址（用于 API Gateway 连接）
	GroupGRPCAddr      string `mapstructure:"group_grpc_addr"`      // 新增：Group Service 地址（用于 API Gateway 连接）
	FriendshipGRPCAddr string `mapstructure:"friendship_grpc_addr"` // 新增：Friendship Service 地址
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
	viper.AddConfigPath(".")
	viper.AddConfigPath("./config")
	viper.AddConfigPath("/root/config") // Docker 容器内的路径

	viper.SetEnvPrefix("CHATIM")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv() // 👈 这个会自动覆盖 config.yaml 中的值

	// 显式绑定环境变量到对应的配置键
	viper.BindEnv("server.user_grpc_addr", "CHATIM_SERVER_USER_GRPC_ADDR")
	viper.BindEnv("server.message_grpc_addr", "CHATIM_SERVER_MESSAGE_GRPC_ADDR")
	viper.BindEnv("server.group_grpc_addr", "CHATIM_SERVER_GROUP_GRPC_ADDR")
	viper.BindEnv("server.friendship_grpc_addr", "CHATIM_SERVER_FRIENDSHIP_GRPC_ADDR")

	if err := viper.ReadInConfig(); err != nil {
		log.Fatalf("Error reading config file, %s", err)
	}

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		log.Fatalf("Error unmarshaling config, %s", err)
	}

	log.Printf("Loaded config - UserGRPCAddr: %s, MessageGRPCAddr: %s", config.Server.UserGRPCAddr, config.Server.MessageGRPCAddr)

	return &config, nil
}
