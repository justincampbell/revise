// Package config handles loading and resolving user configuration for revise.
// Config files are YAML, stored at ~/.config/revise/config.yaml (or
// $XDG_CONFIG_HOME/revise/config.yaml when set).
package config

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config holds the raw parsed config with pointer types so that
// "not set" (nil) is distinguishable from zero values.
type Config struct {
	Theme        *string      `yaml:"theme"`
	DefaultMode  *string      `yaml:"default_mode"`
	ContextLines *int         `yaml:"context_lines"`
	Whitespace   *bool        `yaml:"whitespace"`
	Mouse        *bool        `yaml:"mouse"`
	UpdateCheck  *UpdateCheck `yaml:"update_check"`
}

// UpdateCheck is a custom type that accepts true, false, or "dev" in YAML.
type UpdateCheck string

const (
	UpdateCheckTrue  UpdateCheck = "true"
	UpdateCheckFalse UpdateCheck = "false"
	UpdateCheckDev   UpdateCheck = "dev"
)

func (u *UpdateCheck) UnmarshalYAML(value *yaml.Node) error {
	switch value.Tag {
	case "!!bool":
		// YAML booleans: true/false, yes/no, on/off
		switch strings.ToLower(value.Value) {
		case "true", "yes", "on":
			*u = UpdateCheckTrue
		default:
			*u = UpdateCheckFalse
		}
		return nil
	case "!!str":
		switch strings.ToLower(value.Value) {
		case "true", "yes", "on":
			*u = UpdateCheckTrue
		case "false", "no", "off":
			*u = UpdateCheckFalse
		case "dev":
			*u = UpdateCheckDev
		default:
			return fmt.Errorf("invalid update_check value %q (valid: true, false, dev)", value.Value)
		}
		return nil
	default:
		return fmt.Errorf("invalid update_check value %q (valid: true, false, dev)", value.Value)
	}
}

// ResolvedConfig holds the final config with plain types after merging
// the parsed config over defaults. This is what the rest of the app uses.
type ResolvedConfig struct {
	Theme        string
	DefaultMode  string
	ContextLines int
	Whitespace   bool   // true = show whitespace changes (default)
	Mouse        bool
	UpdateCheck  string // "true", "false", "dev"
}

// Default returns the default resolved config.
func Default() ResolvedConfig {
	return ResolvedConfig{
		Theme:        "dark",
		DefaultMode:  "branch",
		ContextLines: 3,
		Whitespace:   true,
		Mouse:        true,
		UpdateCheck:  "true",
	}
}

// Resolve merges a parsed Config over the defaults. Nil pointer fields
// retain their default values; non-nil fields override them.
func Resolve(cfg Config) ResolvedConfig {
	r := Default()
	if cfg.Theme != nil {
		r.Theme = *cfg.Theme
	}
	if cfg.DefaultMode != nil {
		r.DefaultMode = *cfg.DefaultMode
	}
	if cfg.ContextLines != nil {
		v := *cfg.ContextLines
		if v < 0 {
			v = 0
		} else if v > 20 {
			v = 20
		}
		r.ContextLines = v
	}
	if cfg.Whitespace != nil {
		r.Whitespace = *cfg.Whitespace
	}
	if cfg.Mouse != nil {
		r.Mouse = *cfg.Mouse
	}
	if cfg.UpdateCheck != nil {
		r.UpdateCheck = string(*cfg.UpdateCheck)
	}
	return r
}

// Path returns the resolved config file path.
func Path() string {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "revise", "config.yaml")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "revise", "config.yaml")
}

// Dir returns the directory containing the config file.
func Dir() string {
	p := Path()
	if p == "" {
		return ""
	}
	return filepath.Dir(p)
}

// Load reads and parses a config file. Returns the parsed config,
// a list of warnings (unknown keys, clamped values), and an error.
// If the file does not exist, returns a zero Config with no error.
func Load(path string) (Config, []string, error) {
	var cfg Config
	var warnings []string

	if path == "" {
		return cfg, nil, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return cfg, nil, nil
		}
		return cfg, nil, fmt.Errorf("reading config: %w", err)
	}

	// Strip UTF-8 BOM if present.
	data = bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})

	if len(bytes.TrimSpace(data)) == 0 {
		return cfg, nil, nil
	}

	// First pass: strict decode to catch unknown keys.
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&cfg); err != nil {
		// EOF means the file has no YAML nodes (e.g., all comments).
		if err.Error() == "EOF" {
			return cfg, nil, nil
		}
		// yaml.v3 returns a *yaml.TypeError for unknown fields.
		var typeErr *yaml.TypeError
		if errors.As(err, &typeErr) {
			// Unknown fields — warn but continue. Re-parse without KnownFields.
			for _, msg := range typeErr.Errors {
				warnings = append(warnings, fmt.Sprintf("config: %s", msg))
			}
			cfg = Config{}
			if err := yaml.Unmarshal(data, &cfg); err != nil {
				return Config{}, warnings, fmt.Errorf("parsing config %s: %w", path, err)
			}
		} else {
			return Config{}, nil, fmt.Errorf("parsing config %s: %w", path, err)
		}
	}

	return cfg, warnings, nil
}

// LoadDefault loads the config from the default path.
func LoadDefault() (Config, []string, error) {
	return Load(Path())
}

// Exists returns true if the config file exists at the default path.
func Exists() bool {
	p := Path()
	if p == "" {
		return false
	}
	_, err := os.Stat(p)
	return err == nil
}

// GenerateTemplate returns a commented-out config template with defaults.
func GenerateTemplate() string {
	return `# revise configuration
# Uncomment and edit to customize. Defaults shown.
# See: https://github.com/justincampbell/revise#configuration

# Color theme: dark, light, dark-daltonized, light-daltonized
# theme: dark

# Default diff mode on feature branches.
# Values: branch, staged, staged_only, unstaged_only
# (On the default branch, always starts in staged regardless of this setting)
# default_mode: branch

# Number of context lines around changes (0-20)
# context_lines: 3

# Show whitespace-only changes (set to false to hide them)
# whitespace: true

# Enable mouse support
# mouse: true

# Automatic update checks on startup.
# Values: true, false, "dev" (also check for pre-release/dev builds)
# update_check: true
`
}

// WriteTemplate writes the default config template to the given path,
// creating parent directories as needed. Returns an error if the file
// already exists and force is false.
func WriteTemplate(path string, force bool) error {
	if !force {
		if _, err := os.Stat(path); err == nil {
			return fmt.Errorf("config file already exists at %s (use --force to overwrite)", path)
		}
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}

	if err := os.WriteFile(path, []byte(GenerateTemplate()), 0o644); err != nil {
		return fmt.Errorf("writing config file: %w", err)
	}

	return nil
}
