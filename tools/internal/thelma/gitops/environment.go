package gitops

import "fmt"

// envConfigDir is the subdirectory in terra-helmfile to search for environment config files
const envConfigDir = "environments"

// envNamespacePrefix is the prefix that is added to all environment namespaces.
// Eg. the namespace for the "alpha" environment is "terra-alpha"
const envNamespacePrefix = "terra-"

type Environment interface {
	// Returns the name of the default cluster for this environment. Eg. "terra-qa"
	DefaultCluster() string
	// Returns the namespace for this environment. Eg. "terra-dev"
	Namespace() string
	Target
}

// Environment represents a Terra environment
type environment struct {
	defaultCluster string                // Name of the default cluster for this environment. eg "terra-dev"
	releases       map[string]AppRelease // Set of releases configured in this environment
	target
}

// NewEnvironment constructs a new Environment
func NewEnvironment(name string, base string, defaultCluster string, releases map[string]AppRelease) Environment {
	return &environment{
		defaultCluster: defaultCluster,
		releases:       releases,
		target: target{
			name:       name,
			base:       base,
			targetType: EnvironmentTargetType,
		},
	}
}

func (e *environment) Releases() []Release {
	var result []Release
	for _, r := range e.releases {
		result = append(result, r)
	}
	return result
}

func (e *environment) DefaultCluster() string {
	return e.defaultCluster
}

func (e *environment) ReleaseType() ReleaseType {
	return AppReleaseType
}

// ConfigDir environment configuration subdirectory within terra-helmfile ("environments")
func (e *environment) ConfigDir() string {
	return envConfigDir
}

// Name environment name, eg. "alpha"
func (e *environment) Name() string {
	return e.name
}

// Base environment base, eg. "live"
func (e *environment) Base() string {
	return e.base
}

// Environment namespace. Eg "terra-dev", "terra-perf", etc.
func (e *environment) Namespace() string {
	return environmentNamespace(e.Name())
}

// return environment namespace for a given environment
func environmentNamespace(envName string) string {
	return fmt.Sprintf("%s%s", envNamespacePrefix, envName)
}
