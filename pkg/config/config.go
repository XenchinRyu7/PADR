package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Default limits and configuration values
const (
	DefaultMaxRunsPerDay      = 5
	DefaultMaxRuntimeMinutes  = 45
	DefaultMaxTasksPerRun     = 2
	DefaultMaxCommitsPerRun   = 5
	ProjectConfigDirName      = ".padr"
	ProjectConfigFileName     = "project.yaml"
	GlobalConfigFileName      = "config.yaml"
)

// LimitsConfig defines runtime and execution safety guards
type LimitsConfig struct {
	MaxRunsPerDay      int `yaml:"max_runs_per_day"`
	MaxRuntimeMinutes  int `yaml:"max_runtime_minutes"`
	MaxTasksPerRun     int `yaml:"max_tasks_per_run"`
	MaxCommitsPerRun   int `yaml:"max_commits_per_run"`
}

// ProviderConfig defines model provider credentials and rate limits
type ProviderConfig struct {
	ID           string `yaml:"id"`
	Provider     string `yaml:"provider"`      // groq, google, openrouter, ollama, custom
	Model        string `yaml:"model"`         // model identifier, e.g. openai/gpt-oss-120b
	APIKeyEnv    string `yaml:"api_key_env"`   // environment variable name for api key
	Endpoint     string `yaml:"endpoint"`      // custom endpoint URL for local/custom providers
	MaxDailyRuns int    `yaml:"max_daily_runs"`// daily budget quota for this provider
	Enabled      bool   `yaml:"enabled"`
}

// RoutingConfig defines the model routing and fallback strategy
type RoutingConfig struct {
	Strategy string   `yaml:"strategy"` // e.g. "fallback", "priority"
	Models   []string `yaml:"models"`   // ordered list of provider IDs
}

// GlobalConfig contains all system-wide PADR configurations
type GlobalConfig struct {
	Limits    LimitsConfig              `yaml:"limits"`
	Providers map[string]ProviderConfig `yaml:"providers"`
	Routing   RoutingConfig             `yaml:"routing"`
}

// RepositorySettings defines git repository location and target branch
type RepositorySettings struct {
	Path   string `yaml:"path"`
	Branch string `yaml:"branch"`
}

// AgentSettings defines agent engine configuration
type AgentSettings struct {
	Engine  string `yaml:"engine"`   // default: opencode
	CLIPath string `yaml:"cli_path"` // custom executable path if not in PATH
}

// DevelopmentSettings defines roadmap and scope rules
type DevelopmentSettings struct {
	Roadmap          string `yaml:"roadmap"`           // e.g. ROADMAP.md
	MaxTasks         int    `yaml:"max_tasks"`         // max roadmap tasks per run
	RulesFile        string `yaml:"rules_file"`        // optional PROJECT.md
	ArchitectureFile string `yaml:"architecture_file"` // optional ARCHITECTURE.md
}

// GitSafetySettings controls git automation boundaries
type GitSafetySettings struct {
	AutoPull            bool   `yaml:"auto_pull"`
	AutoCommit          bool   `yaml:"auto_commit"`
	AutoPush            bool   `yaml:"auto_push"`
	CommitMessagePrefix string `yaml:"commit_prefix"`
}

// ValidationSettings defines post-agent verification commands
type ValidationSettings struct {
	Commands []string `yaml:"commands"`
}

// ScheduleSettings defines execution triggers
type ScheduleSettings struct {
	Cron    string `yaml:"cron"`     // e.g. "0 9 * * *"
	DailyAt string `yaml:"daily_at"` // e.g. "09:00" for simplified daily scheduling
}

// ProjectConfig defines autonomous execution for a specific repository
type ProjectConfig struct {
	Name        string              `yaml:"name"`
	Repository  RepositorySettings  `yaml:"repository"`
	Agent       AgentSettings       `yaml:"agent"`
	Development DevelopmentSettings `yaml:"development"`
	Git         GitSafetySettings   `yaml:"git"`
	Validation  ValidationSettings  `yaml:"validation"`
	Schedule    ScheduleSettings    `yaml:"schedule"`
}

// GetPadrHome returns the PADR base directory (~/.padr or PADR_HOME)
func GetPadrHome() (string, error) {
	if custom := os.Getenv("PADR_HOME"); custom != "" {
		return filepath.Clean(custom), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to determine user home directory: %w", err)
	}
	return filepath.Join(home, ".padr"), nil
}

// GetConfigDir returns ~/.padr/config
func GetConfigDir() (string, error) {
	home, err := GetPadrHome()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "config"), nil
}

// GetLogsDir returns ~/.padr/logs
func GetLogsDir() (string, error) {
	home, err := GetPadrHome()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "logs"), nil
}

// GetRunsDir returns ~/.padr/runs
func GetRunsDir() (string, error) {
	home, err := GetPadrHome()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "runs"), nil
}

// GetStateDir returns ~/.padr/state
func GetStateDir() (string, error) {
	home, err := GetPadrHome()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "state"), nil
}

// GetProjectsRegistryDir returns ~/.padr/projects
func GetProjectsRegistryDir() (string, error) {
	home, err := GetPadrHome()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "projects"), nil
}

// DefaultGlobalConfig returns the default configuration structure
func DefaultGlobalConfig() *GlobalConfig {
	return &GlobalConfig{
		Limits: LimitsConfig{
			MaxRunsPerDay:     DefaultMaxRunsPerDay,
			MaxRuntimeMinutes: DefaultMaxRuntimeMinutes,
			MaxTasksPerRun:    DefaultMaxTasksPerRun,
			MaxCommitsPerRun:  DefaultMaxCommitsPerRun,
		},
		Providers: map[string]ProviderConfig{
			"groq-fast": {
				ID:           "groq-fast",
				Provider:     "groq",
				Model:        "openai/gpt-oss-120b",
				APIKeyEnv:    "GROQ_API_KEY",
				MaxDailyRuns: 3,
				Enabled:      true,
			},
			"gemini-fast": {
				ID:           "gemini-fast",
				Provider:     "google",
				Model:        "gemini-2.5-flash",
				APIKeyEnv:    "GEMINI_API_KEY",
				MaxDailyRuns: 3,
				Enabled:      true,
			},
			"openrouter-free": {
				ID:           "openrouter-free",
				Provider:     "openrouter",
				Model:        "meta-llama/llama-3.3-70b-instruct:free",
				APIKeyEnv:    "OPENROUTER_API_KEY",
				MaxDailyRuns: 2,
				Enabled:      true,
			},
			"ollama-local": {
				ID:           "ollama-local",
				Provider:     "ollama",
				Model:        "qwen2.5-coder:7b",
				Endpoint:     "http://localhost:11434",
				MaxDailyRuns: 10,
				Enabled:      true,
			},
		},
		Routing: RoutingConfig{
			Strategy: "fallback",
			Models: []string{
				"groq-fast",
				"gemini-fast",
				"openrouter-free",
				"ollama-local",
			},
		},
	}
}

// InitPadrHome initializes the ~/.padr directory tree and default config
func InitPadrHome() error {
	dirs := []func() (string, error){
		GetConfigDir,
		GetLogsDir,
		GetRunsDir,
		GetStateDir,
		GetProjectsRegistryDir,
	}

	for _, getDir := range dirs {
		dir, err := getDir()
		if err != nil {
			return err
		}
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	cfgPath, err := GetGlobalConfigPath()
	if err != nil {
		return err
	}

	if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
		defaultCfg := DefaultGlobalConfig()
		if err := SaveGlobalConfig(defaultCfg); err != nil {
			return fmt.Errorf("failed to write initial global config: %w", err)
		}
	}

	return nil
}

// GetGlobalConfigPath returns ~/.padr/config/config.yaml
func GetGlobalConfigPath() (string, error) {
	cfgDir, err := GetConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(cfgDir, GlobalConfigFileName), nil
}

// LoadGlobalConfig reads the global configuration file
func LoadGlobalConfig() (*GlobalConfig, error) {
	cfgPath, err := GetGlobalConfigPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(cfgPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Auto initialize if not exists
			if initErr := InitPadrHome(); initErr != nil {
				return nil, initErr
			}
			return DefaultGlobalConfig(), nil
		}
		return nil, fmt.Errorf("failed to read global config at %s: %w", cfgPath, err)
	}

	var cfg GlobalConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse yaml global config: %w", err)
	}

	return &cfg, nil
}

// SaveGlobalConfig persists the global config to disk
func SaveGlobalConfig(cfg *GlobalConfig) error {
	cfgPath, err := GetGlobalConfigPath()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(cfgPath), 0755); err != nil {
		return err
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal global config: %w", err)
	}

	if err := os.WriteFile(cfgPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write global config: %w", err)
	}

	return nil
}

// DefaultProjectConfig creates a default configuration for a given repository
func DefaultProjectConfig(name, repoPath string) *ProjectConfig {
	return &ProjectConfig{
		Name: name,
		Repository: RepositorySettings{
			Path:   filepath.Clean(repoPath),
			Branch: "main",
		},
		Agent: AgentSettings{
			Engine:  "opencode",
			CLIPath: "opencode",
		},
		Development: DevelopmentSettings{
			Roadmap:  "PADR_ROADMAP.md",
			MaxTasks: DefaultMaxTasksPerRun,
		},
		Git: GitSafetySettings{
			AutoPull:            true,
			AutoCommit:          true,
			AutoPush:            true,
			CommitMessagePrefix: "chore(padr):",
		},
		Validation: ValidationSettings{
			Commands: []string{},
		},
		Schedule: ScheduleSettings{
			DailyAt: "09:00",
			Cron:    "0 9 * * *",
		},
	}
}

// LoadProjectConfig loads .padr/project.yaml from the repository directory
func LoadProjectConfig(repoPath string) (*ProjectConfig, error) {
	cfgFile := filepath.Join(repoPath, ProjectConfigDirName, ProjectConfigFileName)
	data, err := os.ReadFile(cfgFile)
	if err != nil {
		return nil, fmt.Errorf("could not read project config at %s: %w", cfgFile, err)
	}

	var cfg ProjectConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("invalid YAML in project config: %w", err)
	}

	return &cfg, nil
}

// SaveProjectConfig writes .padr/project.yaml in the repository
func SaveProjectConfig(repoPath string, cfg *ProjectConfig) error {
	dir := filepath.Join(repoPath, ProjectConfigDirName)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create %s: %w", dir, err)
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal project config: %w", err)
	}

	targetPath := filepath.Join(dir, ProjectConfigFileName)
	if err := os.WriteFile(targetPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write project config: %w", err)
	}

	return nil
}

// RegisterProject saves a project link/spec into ~/.padr/projects/<name>.yaml
func RegisterProject(cfg *ProjectConfig) error {
	regDir, err := GetProjectsRegistryDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(regDir, 0755); err != nil {
		return err
	}

	filePath := filepath.Join(regDir, fmt.Sprintf("%s.yaml", cfg.Name))
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}

	return os.WriteFile(filePath, data, 0644)
}

// ListProjects reads all registered projects from ~/.padr/projects/
func ListProjects() ([]*ProjectConfig, error) {
	regDir, err := GetProjectsRegistryDir()
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(regDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []*ProjectConfig{}, nil
		}
		return nil, err
	}

	var projects []*ProjectConfig
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".yaml" {
			continue
		}

		pPath := filepath.Join(regDir, entry.Name())
		data, err := os.ReadFile(pPath)
		if err != nil {
			continue
		}

		var p ProjectConfig
		if err := yaml.Unmarshal(data, &p); err == nil {
			projects = append(projects, &p)
		}
	}

	return projects, nil
}

// FindProjectByName searches registered projects or tries current working directory
func FindProjectByName(name string) (*ProjectConfig, error) {
	projects, err := ListProjects()
	if err == nil {
		for _, p := range projects {
			if p.Name == name {
				return p, nil
			}
		}
	}

	// Try if name is actually a path to a repo
	if stat, err := os.Stat(name); err == nil && stat.IsDir() {
		cfg, err := LoadProjectConfig(name)
		if err == nil {
			return cfg, nil
		}
	}

	return nil, fmt.Errorf("project '%s' not found", name)
}
