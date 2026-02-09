package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// globalConfig 全局配置实例
var globalConfig *Config

// SetGlobalConfig 设置全局配置
func SetGlobalConfig(cfg *Config) {
	globalConfig = cfg
}

// GetConfig 获取全局配置
func GetConfig() *Config {
	return globalConfig
}

// GetAppConfig 获取应用配置
func GetAppConfig() *AppConfig {
	if globalConfig == nil {
		return &AppConfig{
			Name:    "Auto-Healing",
			Version: "1.0.0",
			URL:     "http://localhost:8080",
			Env:     "production",
		}
	}
	return &globalConfig.App
}

// Config 应用配置结构
type Config struct {
	App      AppConfig      `mapstructure:"app"`
	Server   ServerConfig   `mapstructure:"server"`
	Database DatabaseConfig `mapstructure:"database"`
	Redis    RedisConfig    `mapstructure:"redis"`
	JWT      JWTConfig      `mapstructure:"jwt"`
	Log      LogConfig      `mapstructure:"log"`
}

// AppConfig 应用信息配置
type AppConfig struct {
	Name    string `mapstructure:"name"`
	Version string `mapstructure:"version"`
	URL     string `mapstructure:"url"`
	Env     string `mapstructure:"env"` // production, staging, development
}

// ServerConfig 服务器配置
type ServerConfig struct {
	Host string `mapstructure:"host"`
	Port string `mapstructure:"port"`
	Mode string `mapstructure:"mode"` // debug, release, test
}

// DatabaseConfig 数据库配置
type DatabaseConfig struct {
	Host               string `mapstructure:"host"`
	Port               string `mapstructure:"port"`
	User               string `mapstructure:"user"`
	Password           string `mapstructure:"password"`
	DBName             string `mapstructure:"dbname"`
	SSLMode            string `mapstructure:"ssl_mode"`
	MaxOpenConns       int    `mapstructure:"max_open_conns"`
	MaxIdleConns       int    `mapstructure:"max_idle_conns"`
	MaxLifetimeMinutes int    `mapstructure:"max_lifetime_minutes"`
	LogLevel           string `mapstructure:"log_level"` // silent, error, warn, info
}

// MaxLifetime 返回连接最大生命周期
func (c *DatabaseConfig) MaxLifetime() time.Duration {
	return time.Duration(c.MaxLifetimeMinutes) * time.Minute
}

// RedisConfig Redis 配置
type RedisConfig struct {
	Host     string `mapstructure:"host"`
	Port     string `mapstructure:"port"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
}

// JWTConfig JWT 配置
type JWTConfig struct {
	Secret                string `mapstructure:"secret"`
	AccessTokenTTLMinutes int    `mapstructure:"access_token_ttl_minutes"`
	RefreshTokenTTLHours  int    `mapstructure:"refresh_token_ttl_hours"`
	Issuer                string `mapstructure:"issuer"`
}

// AccessTokenTTL 返回访问令牌过期时间
func (c *JWTConfig) AccessTokenTTL() time.Duration {
	return time.Duration(c.AccessTokenTTLMinutes) * time.Minute
}

// RefreshTokenTTL 返回刷新令牌过期时间
func (c *JWTConfig) RefreshTokenTTL() time.Duration {
	return time.Duration(c.RefreshTokenTTLHours) * time.Hour
}

// LogConfig 日志配置
type LogConfig struct {
	Level   string           `mapstructure:"level"` // debug, info, warn, error
	Console ConsoleLogConfig `mapstructure:"console"`
	File    FileLogConfig    `mapstructure:"file"`
	DBLevel string           `mapstructure:"db_level"` // debug, info, warn, error, off
}

// ConsoleLogConfig 控制台日志配置
type ConsoleLogConfig struct {
	Enabled bool   `mapstructure:"enabled"`
	Format  string `mapstructure:"format"` // text, json
	Color   bool   `mapstructure:"color"`
}

// FileLogConfig 文件日志配置
type FileLogConfig struct {
	Enabled    bool   `mapstructure:"enabled"`
	Path       string `mapstructure:"path"`        // 日志目录
	Filename   string `mapstructure:"filename"`    // 日志文件名
	Format     string `mapstructure:"format"`      // text, json
	MaxSize    int    `mapstructure:"max_size"`    // MB
	MaxBackups int    `mapstructure:"max_backups"` // 保留数量
	MaxAge     int    `mapstructure:"max_age"`     // 天数
	Compress   bool   `mapstructure:"compress"`    // 压缩旧文件
}

// Load 从配置文件加载配置
func Load() (*Config, error) {
	v := viper.New()

	// 设置配置文件路径
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath("./configs")
	v.AddConfigPath(".")

	// 设置默认值
	setDefaults(v)

	// 允许环境变量覆盖
	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// 读取配置文件
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("读取配置文件失败: %w", err)
		}
		// 配置文件不存在时使用默认值
		fmt.Println("📝 未找到配置文件，使用默认配置")
	} else {
		fmt.Printf("📝 已加载配置文件: %s\n", v.ConfigFileUsed())
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("解析配置失败: %w", err)
	}

	return &cfg, nil
}

// setDefaults 设置默认值
func setDefaults(v *viper.Viper) {
	// 应用信息
	v.SetDefault("app.name", "Auto-Healing")
	v.SetDefault("app.version", "1.0.0")
	v.SetDefault("app.url", "http://localhost:8080")
	v.SetDefault("app.env", "production")

	// 服务器
	v.SetDefault("server.host", "0.0.0.0")
	v.SetDefault("server.port", "8080")
	v.SetDefault("server.mode", "debug")

	// 数据库
	v.SetDefault("database.host", "localhost")
	v.SetDefault("database.port", "5432")
	v.SetDefault("database.user", "postgres")
	v.SetDefault("database.password", "postgres")
	v.SetDefault("database.dbname", "auto_healing")
	v.SetDefault("database.ssl_mode", "disable")
	v.SetDefault("database.max_open_conns", 25)
	v.SetDefault("database.max_idle_conns", 5)
	v.SetDefault("database.max_lifetime_minutes", 5)
	v.SetDefault("database.log_level", "info")

	// Redis
	v.SetDefault("redis.host", "localhost")
	v.SetDefault("redis.port", "6379")
	v.SetDefault("redis.password", "")
	v.SetDefault("redis.db", 0)

	// JWT
	v.SetDefault("jwt.secret", "your-super-secret-key-change-in-production")
	v.SetDefault("jwt.access_token_ttl_minutes", 60)
	v.SetDefault("jwt.refresh_token_ttl_hours", 168)
	v.SetDefault("jwt.issuer", "auto-healing")

	// 日志
	v.SetDefault("log.level", "info")
	v.SetDefault("log.console.enabled", true)
	v.SetDefault("log.console.format", "text")
	v.SetDefault("log.console.color", true)
	v.SetDefault("log.file.enabled", true)
	v.SetDefault("log.file.path", "./logs")
	v.SetDefault("log.file.filename", "app.log")
	v.SetDefault("log.file.format", "json")
	v.SetDefault("log.file.max_size", 100)
	v.SetDefault("log.file.max_backups", 10)
	v.SetDefault("log.file.max_age", 30)
	v.SetDefault("log.file.compress", true)
	v.SetDefault("log.db_level", "warn")
}

// DSN 返回数据库连接字符串
func (c *DatabaseConfig) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		c.Host, c.Port, c.User, c.Password, c.DBName, c.SSLMode,
	)
}

// Addr 返回 Redis 地址
func (c *RedisConfig) Addr() string {
	return c.Host + ":" + c.Port
}
