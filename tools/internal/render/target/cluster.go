package target

// clusterConfigDir is the subdirectory in terra-helmfile to search for cluster config files
const clusterConfigDir = "clusters"

// clusterTypeName is the name of the cluster target type, as referenced in the helmfile config repo
const clusterTypeName = "cluster"

// Cluster represents a Terra cluster
type Cluster struct {
	name string // Cluster name. Eg "terra-dev", "terra-alpha", "datarepo-prod"
	base string // Type of cluster. Eg "terra", "datarepo"
}

// NewCluster constructs a new Cluster
func NewCluster(name string, base string) *Cluster {
	return &Cluster{name, base}
}

// NewClusterGeneric like NewCluster, but with a return type of ReleaseTarget
// (this won't be necessary once Go's upcoming support for generic types is available)
func NewClusterGeneric(name string, base string) ReleaseTarget {
	return NewCluster(name, base)
}

// ConfigDir cluster configuration subdirectory within terra-helmfile ("clusters")
func (c *Cluster) ConfigDir() string {
	return clusterConfigDir
}

// Type type name ("cluster")
func (c *Cluster) Type() string {
	return clusterTypeName
}

// Name cluster name, eg. "terra-alpha"
func (c *Cluster) Name() string {
	return c.name
}

// Base cluster base, eg. "terra"
func (c *Cluster) Base() string {
	return c.base
}
