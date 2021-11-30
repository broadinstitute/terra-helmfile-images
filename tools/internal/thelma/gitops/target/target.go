package target

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

// ReleaseTarget represents where a release is being deployed (environment or cluster)
type ReleaseTarget interface {
	ConfigDir() string // ConfigDir returns the subdirectory in the terra-helmfile config repo where environments or clusters are defined
	Type() string      // Type is the name of the target type, either "environment" or "cluster", as referenced in the helmfile repo
	Base() string      // Base is the base of the environment or cluster
	Name() string      // Name is the name of the environment or cluster
}

// SortReleaseTargets sorts release targets lexicographically by type, by base, and then by name
func SortReleaseTargets(targets []ReleaseTarget) {
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

// LoadReleaseTargets loads all environment and cluster release targets from the config repo
func LoadReleaseTargets(configRepoPath string) (map[string]ReleaseTarget, error) {
	targets := make(map[string]ReleaseTarget)

	environments, err := loadEnvironments(configRepoPath)
	if err != nil {
		return nil, err
	}

	clusters, err := loadClusters(configRepoPath)
	if err != nil {
		return nil, err
	}

	for key, env := range environments {
		targets[key] = env
	}

	for key, cluster := range clusters {
		if conflict, ok := targets[key]; ok {
			return nil, fmt.Errorf("cluster name %s conflicts with environment name %s", cluster.Name(), conflict.Name())
		}
		targets[key] = cluster
	}

	return targets, nil
}

// loadEnvironments scans through the environments/ subdirectory and build a slice of defined environments
func loadEnvironments(configRepoPath string) (map[string]ReleaseTarget, error) {
	configDir := path.Join(configRepoPath, envConfigDir)

	return loadTargetsFromDirectory(configDir, envTypeName, NewEnvironmentGeneric)
}

// loadClusters scans through the cluster/ subdirectory and build a slice of defined clusters
func loadClusters(configRepoPath string) (map[string]ReleaseTarget, error) {
	configDir := path.Join(configRepoPath, clusterConfigDir)

	return loadTargetsFromDirectory(configDir, clusterTypeName, NewClusterGeneric)
}

// loadTargetsFromDirectory loads the set of configured clusters or environments from a config directory
func loadTargetsFromDirectory(configDir string, targetType string, constructor func(string, string) ReleaseTarget) (map[string]ReleaseTarget, error) {
	targets := make(map[string]ReleaseTarget)

	if _, err := os.Stat(configDir); err != nil {
		return nil, fmt.Errorf("%s config directory does not exist: %s", targetType, configDir)
	}

	matches, err := filepath.Glob(path.Join(configDir, "*", "*.yaml"))
	if err != nil {
		return nil, fmt.Errorf("error loading %s configs from %s: %v", targetType, configDir, err)
	}

	if len(matches) == 0 {
		return nil, fmt.Errorf("no %s configs found in %s", targetType, configDir)
	}

	for _, filename := range matches {
		base := path.Base(path.Dir(filename))
		name := strings.TrimSuffix(path.Base(filename), ".yaml")

		target := constructor(name, base)

		if conflict, ok := targets[target.Name()]; ok {
			return nil, fmt.Errorf("%s name conflict %s (%s) and %s (%s)", targetType, target.Name(), target.Base(), conflict.Name(), conflict.Base())
		}

		targets[target.Name()] = target
	}

	return targets, nil
}
