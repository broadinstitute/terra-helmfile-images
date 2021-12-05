package config

// Data is a mutable, serializable struct for building an immutable Config
type Data struct {
	Home     string `yaml:"home"`
	Tmpdir   string `yaml:"tmpdir"`
	LogLevel string `yaml:"loglevel"`
}

// Keys makes yaml serialization keys for Data fields available without reflection.
var Keys = struct {
	Home     string
	Tmpdir   string
	LogLevel string
}{
	Home:     "home",
	Tmpdir:   "tmpdir",
	LogLevel: "loglevel",
}

// Config represents global config for Thelma
type Config struct {
	data Data
}

// New constructs a new config
func New(data Data) *Config {
	return &Config{data: data}
}

// Home is the path to a terra-helmfile clone
func (cfg *Config) Home() string {
	return cfg.data.Home
}

// LogLevel is the level at which Thelma should log
func (cfg *Config) LogLevel() string {
	return cfg.data.LogLevel
}

// Tmpdir directory where Thelma should create temporary files
func (cfg *Config) Tmpdir() string {
	return cfg.data.Tmpdir
}
