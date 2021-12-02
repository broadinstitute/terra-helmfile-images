package gitops

type Cluster interface {
	Address() string
	Target
}

// clusterConfigDir is the subdirectory in terra-helmfile to search for cluster config files
const clusterConfigDir = "clusters"

// Cluster represents a Terra cluster
type cluster struct {
	address  string // Cluster API address. Eg "https://10.0.0.1/api"
	releases map[string]ClusterRelease
	target
}

// NewCluster constructs a new Cluster
func NewCluster(name string, base string, address string, releases map[string]ClusterRelease) Cluster {
	return &cluster{
		address:  address,
		releases: releases,
		target: target{
			name:       name,
			base:       base,
			targetType: ClusterTargetType,
		},
	}
}

func (c *cluster) Releases() []Release {
	var result []Release
	for _, r := range c.releases {
		result = append(result, r)
	}
	return result
}

func (c *cluster) Address() string {
	return c.address
}

func (c *cluster) ReleaseType() ReleaseType {
	return ClusterReleaseType
}

// ConfigDir cluster configuration subdirectory within terra-helmfile ("clusters")
func (c *cluster) ConfigDir() string {
	return clusterConfigDir
}

// Name cluster name, eg. "terra-alpha"
func (c *cluster) Name() string {
	return c.name
}

// Base cluster base, eg. "terra"
func (c *cluster) Base() string {
	return c.base
}
