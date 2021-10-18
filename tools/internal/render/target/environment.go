package target

// envConfigDir is the subdirectory in terra-helmfile to search for environment config files
const envConfigDir = "environments"

// envTypeName is the name of the environment target type, as referenced in the helmfile config repo
const envTypeName = "environment"

// Environment represents a Terra environment
type Environment struct {
	name string // Environment name. Eg "dev", "alpha", "prod"
	base string // Type of environment. Eg "live", "personal"
}

// NewEnvironment constructs a new Environment
func NewEnvironment(name string, base string) *Environment {
	return &Environment{name, base}
}

// NewEnvironmentGeneric like NewEnvironment, but with a return type of ReleaseTarget
// (this won't be necessary once Go's upcoming support for generic types is available)
func NewEnvironmentGeneric(name string, base string) ReleaseTarget {
	return NewEnvironment(name, base)
}

// ConfigDir environment configuration subdirectory within terra-helmfile ("environments")
func (e *Environment) ConfigDir() string {
	return envConfigDir
}

// Type type name ("environment")
func (e *Environment) Type() string {
	return envTypeName
}

// Name environment name, eg. "alpha"
func (e *Environment) Name() string {
	return e.name
}

// Base environment base, eg. "live"
func (e *Environment) Base() string {
	return e.base
}
