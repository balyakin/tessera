package config

import _ "embed"

//go:embed default.toml
var defaultTOML string

func DefaultTOML() string {
	return defaultTOML
}
