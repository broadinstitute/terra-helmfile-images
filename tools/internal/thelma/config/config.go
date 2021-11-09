package config

// Mutable, serializable struct for building an immutable Config
type ConfigData struct {
	Home     string `yaml:"home"`
	Tmpdir   string `yaml:"tmpdir"`
	LogLevel string `yaml:"loglevel"`
}

// Make yaml serialization keys for ConfigData fields available without reflection.
var ConfigKeys = struct {
	Home     string
	Tmpdir   string
	LogLevel string
}{
	Home:     "home",
	Tmpdir:   "tmpdir",
	LogLevel: "loglevel",
}

// Immutable Config object
type Config struct {
	data ConfigData
}

func New(data ConfigData) *Config {
	return &Config{data: data}
}

func (cfg *Config) Home() string {
	return cfg.data.Home
}

func (cfg *Config) LogLevel() string {
	return cfg.data.LogLevel
}

func (cfg *Config) Tmpdir() string {
	return cfg.data.Tmpdir
}
