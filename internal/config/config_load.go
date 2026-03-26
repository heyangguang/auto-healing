package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	appcrypto "github.com/company/auto-healing/internal/pkg/crypto"
	"github.com/spf13/viper"
)

const insecureJWTSecretPlaceholder = "your-super-secret-key-change-in-production"

func load(requireFile bool) (*Config, error) {
	v := viper.New()
	configureConfigLookup(v)
	setDefaults(v)
	configureEnvOverrides(v)

	source, err := readConfig(v, requireFile)
	if err != nil {
		return nil, err
	}

	cfg, err := unmarshal(v)
	if err != nil {
		return nil, err
	}
	if err := ensureJWTSecret(&cfg, source); err != nil {
		return nil, err
	}
	warnProductionConfig(&cfg)
	return &cfg, nil
}

func configureConfigLookup(v *viper.Viper) {
	if file := strings.TrimSpace(os.Getenv("AUTO_HEALING_CONFIG_FILE")); file != "" {
		v.SetConfigFile(file)
		return
	}

	v.SetConfigName("config")
	v.SetConfigType("yaml")
	for _, path := range configSearchPaths() {
		v.AddConfigPath(path)
	}
}

func configureEnvOverrides(v *viper.Viper) {
	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
}

func readConfig(v *viper.Viper, requireFile bool) (string, error) {
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return "", fmt.Errorf("读取配置文件失败: %w", err)
		}
		if requireFile {
			return "", fmt.Errorf("未找到配置文件：请通过 AUTO_HEALING_CONFIG_FILE 指定，或在可执行文件目录/工作目录附近提供 config.yaml")
		}
		fmt.Println("📝 未找到配置文件，使用默认配置")
		return "defaults+env", nil
	}

	source := v.ConfigFileUsed()
	fmt.Printf("📝 已加载配置文件: %s\n", source)
	return source, nil
}

func unmarshal(v *viper.Viper) (Config, error) {
	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return Config{}, fmt.Errorf("解析配置失败: %w", err)
	}
	return cfg, nil
}

func ensureJWTSecret(cfg *Config, source string) error {
	if cfg.JWT.Secret != "" && cfg.JWT.Secret != insecureJWTSecretPlaceholder {
		return nil
	}
	if cfg.App.Env == "production" {
		return fmt.Errorf("配置校验失败 [%s]: production 环境必须显式配置 jwt.secret，当前值为空或仍为示例占位符", source)
	}

	secret, err := appcrypto.GenerateRandomString(48)
	if err != nil {
		return fmt.Errorf("生成默认 JWT Secret 失败: %w", err)
	}
	cfg.JWT.Secret = secret
	fmt.Println("⚠️  未配置安全的 JWT Secret，已生成临时随机值；重启后会失效，请尽快在配置中显式设置 jwt.secret")
	return nil
}

func warnProductionConfig(cfg *Config) {
	if cfg.App.Env != "production" {
		return
	}
	if cfg.Server.Mode == "debug" {
		fmt.Println("⚠️  当前 app.env=production 但 server.mode=debug，请尽快切换到 release")
	}
	if cfg.Database.SSLMode == "disable" {
		fmt.Println("⚠️  当前 app.env=production 但 database.ssl_mode=disable，请确认数据库链路已启用 TLS")
	}
	if cfg.Database.Password == "postgres" {
		fmt.Println("⚠️  当前 app.env=production 仍在使用默认数据库密码 postgres，请立即更换")
	}
}

func configSearchPaths() []string {
	candidates := []string{"./configs", "."}
	execPath, err := os.Executable()
	if err == nil {
		dir := filepath.Dir(execPath)
		parent := filepath.Dir(dir)
		candidates = append(candidates,
			filepath.Join(dir, "configs"),
			dir,
			filepath.Join(parent, "configs"),
			parent,
		)
	}
	return dedupePaths(candidates)
}

func dedupePaths(paths []string) []string {
	result := make([]string, 0, len(paths))
	seen := make(map[string]struct{}, len(paths))
	for _, path := range paths {
		clean := filepath.Clean(strings.TrimSpace(path))
		if clean == "" {
			continue
		}
		if _, ok := seen[clean]; ok {
			continue
		}
		seen[clean] = struct{}{}
		result = append(result, clean)
	}
	return result
}
