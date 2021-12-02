package gitops

import "strings"

type Release interface {
	Name() string
	Type() ReleaseType
	IsAppRelease() bool
	IsClusterRelease() bool
	ChartVersion() string
	ChartName() string
	Repo() string
	Namespace() string
	ClusterName() string
	ClusterAddress() string
	Target() Target
	// Returns 0 if r == other, -1 if r < other, or +1 if r > other.
	Compare(Release) int
}

type release struct {
	name           string
	releaseType    ReleaseType
	chartVersion   string
	chartName      string
	repo           string
	namespace      string
	clusterName    string
	clusterAddress string
	target         Target
}

func (r *release) Name() string {
	return r.name
}

func (r *release) Type() ReleaseType {
	return r.releaseType
}

func (r *release) IsAppRelease() bool {
	return r.Type() == AppReleaseType
}

func (r *release) IsClusterRelease() bool {
	return r.Type() == ClusterReleaseType
}

func (r *release) ChartName() string {
	return r.chartName
}

func (r *release) ChartVersion() string {
	return r.chartVersion
}

func (r *release) Repo() string {
	return r.repo
}

func (r *release) Namespace() string {
	return r.namespace
}

func (r *release) ClusterName() string {
	return r.clusterName
}

func (r *release) ClusterAddress() string {
	return r.clusterAddress
}

func (r *release) Target() Target {
	return r.target
}

// Returns 0 if r == other, -1 if r < other, or +1 if r > other.
// Compares by type, then by name, then by target
func (r *release) Compare(other Release) int {
	byType := r.Type().Compare(other.Type())
	if byType != 0 {
		return byType
	}
	byName := strings.Compare(r.Name(), other.Name())
	if byName != 0 {
		return byName
	}
	byTarget := r.Target().Compare(other.Target())
	return byTarget
}
