package gitops

import (
	"fmt"
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/thelma/gitops/deepmerge"
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/thelma/gitops/serializers"
	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v3"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

// Default settings file name for both types of targets
const defaultsFileName = "defaults.yaml"
const defaultChartRepo = "terra-helm"
const yamlSuffix = ".yaml"

// Target represents where a release is being deployed (environment or cluster)
type Target interface {
	ConfigDir() string        // ConfigDir returns the subdirectory in the terra-helmfile config repo where environments or clusters are defined
	Type() TargetType         // Type is the name of the target type, either "environment" or "cluster", as referenced in the helmfile repo
	Base() string             // Base is the base of the environment or cluster
	Name() string             // Name is the name of the environment or cluster
	ReleaseType() ReleaseType // ReleaseType returns the types of releases that can be deployed to this target
	Releases() []Release      // Releases returns the set of releases configured for this target
	IsCluster() bool          // Returns true if this target is a cluster
	IsEnvironment() bool      // Returns true if this target is an environment
	Compare(other Target) int // Returns 0 if t == other, -1 if t < other, or +1 if t > other.
}

type target struct {
	name       string
	base       string
	targetType TargetType
}

func (t *target) Name() string {
	return t.name
}

func (t *target) Base() string {
	return t.base
}

func (t *target) Type() TargetType {
	return t.targetType
}

func (t *target) IsCluster() bool {
	return t.targetType == ClusterTargetType
}

func (t *target) IsEnvironment() bool {
	return t.targetType == EnvironmentTargetType
}

// Returns 0 if t == other, -1 if t < other, or +1 if t > other.
// Compares lexicographically by type, by base, and then by name.
func (t *target) Compare(other Target) int {
	byType := t.Type().Compare(other.Type())
	if byType != 0 {
		return byType
	}
	byBase := strings.Compare(t.Base(), other.Base())
	if byBase != 0 {
		return byBase
	}
	byName := strings.Compare(t.Name(), other.Name())
	return byName
}

// SortReleaseTargets sorts release targets lexicographically by type, by base, and then by name
func SortReleaseTargets(targets []Target) {
	sort.Slice(targets, func(i int, j int) bool {
		return targets[i].Compare(targets[j]) < 0
	})
}

// LoadEnvironments scans through the environments/ subdirectory and build a slice of defined environments
func LoadEnvironments(configRepoPath string, versions Versions, clusters map[string]Cluster) (map[string]Environment, error) {
	configDir := path.Join(configRepoPath, envConfigDir)

	targetDefs, err := loadTargetsFromDirectory(configDir, EnvironmentTargetType)
	if err != nil {
		return nil, err
	}

	result := make(map[string]Environment)

	for _, targetDef := range targetDefs {
		if cluster, exists := clusters[targetDef.name]; exists {
			return nil, fmt.Errorf("cluster name %s conflicts with environment name %s", cluster.Name(), targetDef.name)
		}
		env, err := loadEnvironment(targetDef, versions, clusters)
		if err != nil {
			return nil, err
		}
		result[env.Name()] = env
	}

	return result, nil
}

func loadEnvironment(targetDef targetDefinition, _versions Versions, clusters map[string]Cluster) (Environment, error) {
	envName := targetDef.name
	envBase := targetDef.base

	log.Debug().Msgf("Loading environment %s", envName)

	var envDefn serializers.Environment
	err := yaml.Unmarshal(targetDef.mergedYaml, &envDefn)
	if err != nil {
		return nil, fmt.Errorf("error reading configuration for environment %s: %v", envName, err)
	}

	defaultClusterName := envDefn.DefaultCluster
	if defaultClusterName == "" {
		return nil, fmt.Errorf("environment %s does not have a valid default cluster", envName)
	}
	defaultCluster, exists := clusters[defaultClusterName]
	if !exists {
		return nil, fmt.Errorf("environment %s: default cluster %s is not defined in %s directory", envName, defaultClusterName, clusterConfigDir)
	}

	releases := make(map[string]AppRelease)

	for releaseName, releaseDefn := range envDefn.Releases {
		log.Debug().Msgf("Processing environment %s release %s: %v", envName, releaseName, releaseDefn)

		// Skip releases that aren't enabled
		if !releaseDefn.Enabled {
			log.Debug().Msgf("environment %s: ignoring disabled release %s", envName, releaseName)
			continue
		}

		_cluster := defaultCluster
		if releaseDefn.Cluster != "" {
			if _, exists := clusters[releaseDefn.Cluster]; !exists {
				return nil, fmt.Errorf("environment %s: release %s: cluster %s is not defined in %s directory", envName, releaseName, releaseDefn.Cluster, clusterConfigDir)
			}
			_cluster = clusters[releaseDefn.Cluster]
		}
		clusterName := _cluster.Name()
		clusterAddress := _cluster.Address()

		// chart version
		chartVersion := releaseDefn.ChartVersion
		if chartVersion == "" {
			chartVersion = _versions.GetSnapshot(AppReleaseType, versionSetFor(envName)).ChartVersion(releaseName)
		}
		if chartVersion == "" {
			chartVersion = _versions.GetSnapshot(AppReleaseType, Dev).ChartVersion(releaseName)
		}
		if chartVersion == "" {
			return nil, fmt.Errorf("environment %s: could not identify chart version for release %s", envName, releaseName)
		}

		// app version
		appVersion := releaseDefn.AppVersion
		if appVersion == "" {
			appVersion = _versions.GetSnapshot(AppReleaseType, versionSetFor(envName)).AppVersion(releaseName)
		}
		if appVersion == "" {
			appVersion = _versions.GetSnapshot(AppReleaseType, Dev).AppVersion(releaseName)
		}
		if appVersion == "" {
			return nil, fmt.Errorf("environment %s: could not identify app version for release %s", envName, releaseName)
		}

		// namespace
		// eg. terra-dev
		namespace := environmentNamespace(envName)

		// chartName
		chartName := releaseDefn.ChartName
		if chartName == "" {
			// chart name defaults to release name if it is not set
			chartName = releaseName
		}

		// repo
		repo := releaseDefn.Repo
		if repo == "" {
			repo = defaultChartRepo
		}

		_release := &appRelease{
			appVersion: appVersion,
			release: release{
				name:           releaseName,
				releaseType:    AppReleaseType,
				chartVersion:   chartVersion,
				chartName:      chartName,
				repo:           repo,
				namespace:      namespace,
				clusterName:    clusterName,
				clusterAddress: clusterAddress,
			},
		}

		releases[releaseName] = _release
	}

	log.Debug().Msgf("Found %d releases for environment %s", len(releases), envName)

	env := NewEnvironment(
		envName,
		envBase,
		defaultClusterName,
		releases,
	)

	for _, _release := range releases {
		_release.(*appRelease).target = env
	}

	return env, nil
}

// LoadClusters scans through the cluster/ subdirectory and build a slice of defined clusters
func LoadClusters(configRepoPath string, versions Versions) (map[string]Cluster, error) {
	configDir := path.Join(configRepoPath, clusterConfigDir)

	targetDefs, err := loadTargetsFromDirectory(configDir, ClusterTargetType)
	if err != nil {
		return nil, err
	}

	result := make(map[string]Cluster)

	for _, targetDef := range targetDefs {
		_cluster, err := loadCluster(targetDef, versions)
		if err != nil {
			return nil, err
		}
		result[_cluster.Name()] = _cluster
	}

	return result, nil
}

func loadCluster(targetDef targetDefinition, _versions Versions) (Cluster, error) {
	clusterName := targetDef.name
	clusterBase := targetDef.base

	var clusterDefn serializers.Cluster
	err := yaml.Unmarshal(targetDef.mergedYaml, &clusterDefn)
	if err != nil {
		return nil, fmt.Errorf("error reading configuration for cluster %s: %v", clusterName, err)
	}

	clusterAddress := clusterDefn.Address
	if clusterAddress == "" {
		return nil, fmt.Errorf("cluster %s does not have a valid API address, please set `address` key in config file", clusterName)
	}

	releases := make(map[string]ClusterRelease)

	for releaseName, releaseDefn := range clusterDefn.Releases {
		// Skip releaes that aren't enabled
		if !releaseDefn.Enabled {
			log.Debug().Msgf("cluster %s: ignoring disabled release %s", clusterName, releaseName)
			continue
		}

		// Release is enabled, so configure with proper settings
		// chart version
		chartVersion := releaseDefn.ChartVersion
		if chartVersion == "" {
			chartVersion = _versions.GetSnapshot(ClusterReleaseType, versionSetFor(clusterName)).ChartVersion(releaseName)
		}
		if chartVersion == "" {
			chartVersion = _versions.GetSnapshot(ClusterReleaseType, Dev).ChartVersion(releaseName)
		}
		if chartVersion == "" {
			return nil, fmt.Errorf("cluster %s: could not identify chart version for release %s", clusterName, releaseName)
		}

		// namespace
		namespace := releaseDefn.Namespace
		if namespace == "" {
			return nil, fmt.Errorf("cluster %s: release %s does not have a valid namespace", clusterName, releaseName)
		}

		// chartName
		chartName := releaseDefn.ChartName
		if chartName == "" {
			// chart name defaults to release name if it is not set
			chartName = releaseName
		}

		// repo
		repo := releaseDefn.Repo
		if repo == "" {
			repo = defaultChartRepo
		}

		releases[releaseName] = &clusterRelease{
			release: release{
				name:           releaseName,
				releaseType:    ClusterReleaseType,
				chartVersion:   chartVersion,
				chartName:      chartName,
				repo:           repo,
				namespace:      namespace,
				clusterName:    clusterName,
				clusterAddress: clusterAddress,
			},
		}
	}

	_cluster := NewCluster(
		clusterName,
		clusterBase,
		clusterDefn.Address,
		releases,
	)

	for _, _release := range releases {
		_release.(*clusterRelease).target = _cluster
	}

	return _cluster, nil
}

// Silly heuristic... If the target name ends with "alpha", use the alpha snapshot, etc.
// Defaults to dev snapshot.
func versionSetFor(targetName string) VersionSet {
	for _, versionSet := range VersionSets() {
		if strings.HasSuffix(targetName, versionSet.String()) {
			return versionSet
		}
	}

	return Dev
}

type targetDefinition struct {
	name       string
	base       string
	mergedYaml []byte
}

// loadTargetsFromDirectory loads the set of configured clusters or environments from a config directory
func loadTargetsFromDirectory(configDir string, targetType TargetType) (map[string]targetDefinition, error) {
	targetDefs := make(map[string]targetDefinition)

	if _, err := os.Stat(configDir); err != nil {
		return nil, fmt.Errorf("%s config directory does not exist: %s", targetType, configDir)
	}

	matches, err := filepath.Glob(path.Join(configDir, "*", withYamlSuffix("*")))
	if err != nil {
		return nil, fmt.Errorf("error loading %s configs from %s: %v", targetType, configDir, err)
	}

	if len(matches) == 0 {
		return nil, fmt.Errorf("no %s configs found in %s", targetType, configDir)
	}

	for _, filename := range matches {
		base := path.Base(path.Dir(filename))
		name := strings.TrimSuffix(path.Base(filename), yamlSuffix)

		if conflict, ok := targetDefs[name]; ok {
			return nil, fmt.Errorf("%s name conflict %s (%s) and %s (%s)", targetType, name, base, conflict.name, conflict.base)
		}

		mergedYaml, err := mergeTargetYaml(configDir, base, name)
		if err != nil {
			return nil, fmt.Errorf("error loading YAML config for %s %s: %v", targetType, name, err)
		}

		targetDef := targetDefinition{
			name:       name,
			base:       base,
			mergedYaml: mergedYaml,
		}

		targetDefs[name] = targetDef
	}

	return targetDefs, nil
}

func mergeTargetYaml(configDir string, base string, name string) ([]byte, error) {
	defaultsFile := path.Join(configDir, defaultsFileName)         // eg. environments/defaults.yaml
	baseFile := path.Join(configDir, withYamlSuffix(base))         // eg. environments/live.yaml
	targetFile := path.Join(configDir, base, withYamlSuffix(name)) // eg. environments/live/dev.yaml

	log.Debug().Msgf("Deep merging: %s, %s, %s", defaultsFile, baseFile, targetFile)
	return deepmerge.Merge(defaultsFile, baseFile, targetFile)
}

func withYamlSuffix(baseName string) string {
	return fmt.Sprintf("%s%s", baseName, yamlSuffix)
}
