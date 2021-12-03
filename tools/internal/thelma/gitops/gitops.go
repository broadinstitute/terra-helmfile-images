package gitops

import (
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/thelma/utils/shell"
	"sort"
)

type Gitops interface {
	Targets() []Target
	Releases() []Release
	FilterReleases(ReleaseFilter) []Release
	GetTarget(name string) Target
}

type ReleaseFilter interface {
	Matches(Release) bool
	And(ReleaseFilter) ReleaseFilter
	Or(ReleaseFilter) ReleaseFilter
}

type gitops struct {
	versions     Versions
	environments map[string]Environment
	clusters     map[string]Cluster
}

type filter struct {
	matcher func(Release) bool
}

func (f *filter) Matches(release Release) bool {
	return f.matcher(release)
}

func HasName(releaseName string) ReleaseFilter {
	return &filter{
		matcher: func(r Release) bool {
			return r.Name() == releaseName
		},
	}
}

func HasTarget(targetName string) ReleaseFilter {
	return &filter{
		matcher: func(r Release) bool {
			return r.Target().Name() == targetName
		},
	}
}

func AnyRelease() ReleaseFilter {
	return &filter{
		matcher: func(_ Release) bool {
			return true
		},
	}
}

func (f *filter) And(other ReleaseFilter) ReleaseFilter {
	return &filter{
		matcher: func(release Release) bool {
			return f.Matches(release) && other.Matches(release)
		},
	}
}

func (f *filter) Or(other ReleaseFilter) ReleaseFilter {
	return &filter{
		matcher: func(release Release) bool {
			return f.Matches(release) || other.Matches(release)
		},
	}
}

func Load(thelmaHome string, shellRunner shell.Runner) (Gitops, error) {
	_versions, err := NewVersions(thelmaHome, shellRunner)
	if err != nil {
		return nil, err
	}

	clusters, err := LoadClusters(thelmaHome, _versions)
	if err != nil {
		return nil, err
	}

	environments, err := LoadEnvironments(thelmaHome, _versions, clusters)
	if err != nil {
		return nil, err
	}

	return &gitops{
		versions:     _versions,
		clusters:     clusters,
		environments: environments,
	}, nil
}

func (g *gitops) Releases() []Release {
	return g.FilterReleases(AnyRelease())
}

func (g *gitops) FilterReleases(f ReleaseFilter) []Release {
	var result []Release
	for _, _target := range g.Targets() {
		for _, _release := range _target.Releases() {
			if f.Matches(_release) {
				result = append(result, _release)
			}
		}
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Compare(result[j]) < 0
	})

	return result
}

func (g *gitops) Targets() []Target {
	var result []Target
	for _, env := range g.environments {
		result = append(result, env)
	}
	for _, cluster := range g.clusters {
		result = append(result, cluster)
	}

	SortReleaseTargets(result)

	return result
}

func (g *gitops) GetTarget(name string) Target {
	if _target, exists := g.clusters[name]; exists {
		return _target
	}

	if _target, exists := g.environments[name]; exists {
		return _target
	}

	return nil
}
