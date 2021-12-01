package gitops

type Environment interface {
	DefaultCluster() string
	Target
}

// envConfigDir is the subdirectory in terra-helmfile to search for environment config files
const envConfigDir = "environments"

// envTypeName is the name of the environment target type, as referenced in the helmfile config repo
const envTypeName = "environment"

// Environment represents a Terra environment
type environment struct {
	defaultCluster string          // Name of the default cluster for this environment. eg "terra-dev"
	releases map[string]AppRelease // Set of releases configured in this environment
	target
}

// NewEnvironment constructs a new Environment
func NewEnvironment(name string, base string, defaultCluster string, releases map[string]AppRelease) Environment {
	return &environment{
		defaultCluster: defaultCluster,
		releases: releases,
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
