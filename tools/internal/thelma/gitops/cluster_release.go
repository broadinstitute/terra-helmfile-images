package gitops

type ClusterRelease interface {
	Cluster() Cluster
	Release
}

type clusterRelease struct {
	release
}

func (r *clusterRelease) Cluster() Cluster {
	return r.target.(Cluster)
}
