package config

import (
	"os"

	"github.com/pelletier/go-toml/v2"
)

type Config struct {
	Tasks []Task `toml:"tasks"`
}

type Task struct {
	Name           string `toml:"name"`
	Cron           string `toml:"cron"`
	SourceType     string `toml:"source_type"` // local, sftp, ftp
	SourcePath     string `toml:"source_path"`
	SourceRegex    string `toml:"source_regex"`
	TargetType     string `toml:"target_type"` // local, sftp, ftp
	TargetPath     string `toml:"target_path"`
	RetentionDays  int    `toml:"retention_days"` // 清理多少天之前的文件
	SourceNewerDays int   `toml:"source_newer_days"` // 仅遍历多少天内的文件
	SourceAuth     *Auth  `toml:"source_auth,omitempty"`
	TargetAuth     *Auth  `toml:"target_auth,omitempty"`
}

type Auth struct {
	Host     string `toml:"host"`
	Port     int    `toml:"port"`
	User     string `toml:"user"`
	Password string `toml:"password"`
}

func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	err = toml.Unmarshal(data, &cfg)
	return &cfg, err
}
