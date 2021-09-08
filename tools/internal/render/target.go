package render

import (
	"sort"
)

// envConfigDir is the subdirectory to search for environment config files
const envConfigDir = "environments"
// envTypeName is the name of the environment target type, as referenced in the helmfile config repo
const envTypeName = "environment"

// clusterConfigDir is the subdirectory to search for cluster config files
const clusterConfigDir = "clusters"
// clusterTypeName is the name of the cluster target type, as referenced in the helmfile config repo
const clusterTypeName = "cluster"

// ReleaseTarget represents where a release is being deployed (environment or cluster)
type ReleaseTarget interface {
	ConfigDir() string // ConfigDir returns the subdirectory in the terra-helmfile config repo where environments or clusters are defined
	Type() string // Type is the name of the target type, either "environment" or "cluster", as referenced in the helmfile repo
	Base() string // Base is the base of the environment or cluster
	Name() string // Name is the name of the environment or cluster
}

// sortReleaseTargets sorts release targets lexicographically by type, by base, and then by name
func sortReleaseTargets(targets []ReleaseTarget) {
	sort.Slice(targets, func(i int, j int) bool {
		if targets[i].Type() != targets[j].Type() {
			return targets[i].Type() < targets[j].Type()
		}
		if targets[i].Base() != targets[j].Base() {
			return targets[i].Base() < targets[j].Base()
		}
		return targets[i].Name() < targets[j].Name()
	})
}

// Environment represents a Terra environment
type Environment struct {
	name string // Environment name. Eg "dev", "alpha", "prod"
	base string // Type of environment. Eg "live", "personal"
}

func NewEnvironment(name string, base string) *Environment {
	return &Environment{name, base}
}

func NewEnvironmentGeneric(name string, base string) ReleaseTarget {
	return NewEnvironment(name, base)
}

func (e *Environment) ConfigDir() string {
	return envConfigDir
}

func (e *Environment) Type() string {
	return envTypeName
}

func (e *Environment) Name() string {
	return e.name
}

func (e *Environment) Base() string {
	return e.base
}

// Cluster represents a Terra cluster
type Cluster struct {
	name string // Cluster name. Eg "terra-dev", "terra-alpha", "datarepo-prod"
	base string // Type of cluster. Eg "terra", "datarepo"
}

func NewCluster(name string, base string) *Cluster {
	return &Cluster{name, base}
}

func NewClusterGeneric(name string, base string) ReleaseTarget {
	return NewCluster(name, base)
}

func (c *Cluster) ConfigDir() string {
	return clusterConfigDir
}

func (c *Cluster) Type() string {
	return clusterTypeName
}

func (c *Cluster) Name() string {
	return c.name
}

func (c *Cluster) Base() string {
	return c.base
}