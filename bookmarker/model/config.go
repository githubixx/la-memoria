package model

import (
	"regexp"
)

var environmentNamePattern = regexp.MustCompile(`^[A-Z_][A-Z0-9_]*$`)

type Configuration struct {
	Administrator Administrator    `yaml:"administrator"`
	Branding      BrandingConfig   `yaml:"branding"`
	DefaultView   string           `yaml:"default_view"`
	Database      DatabaseConfig   `yaml:"database"`
	Screenshots   ScreenshotConfig `yaml:"screenshots"`
	Search        SearchConfig     `yaml:"search"`
	Server        ServerConfig     `yaml:"server"`
}

type BrandingConfig struct {
	PageTitle   string `yaml:"page_title" json:"page_title"`
	FaviconPath string `yaml:"favicon_path" json:"favicon_path"`
}
type DatabaseConfig struct {
	Host        string `yaml:"host" json:"host"`
	Port        int    `yaml:"port" json:"port"`
	Name        string `yaml:"name" json:"name"`
	User        string `yaml:"user" json:"user"`
	PasswordEnv string `yaml:"password_env" json:"password_env"`
	Password    string `yaml:"-" json:"-"`
	TLSMode     string `yaml:"tls_mode" json:"tls_mode"`
}
type ScreenshotConfig struct {
	Root string `yaml:"root" json:"root"`
}
type SearchConfig struct {
	PageSize       int `yaml:"page_size" json:"page_size"`
	MaximumResults int `yaml:"maximum_results" json:"maximum_results"`
}
type ServerConfig struct {
	ListenAddress     string   `yaml:"listen_address"`
	TrustedProxyCIDRs []string `yaml:"trusted_proxy_cidrs"`
}
