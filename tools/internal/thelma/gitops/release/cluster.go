package release

type ClusterRelease struct {

}

func (r *ClusterRelease) Type() ReleaseType {
	return ClusterType
}
