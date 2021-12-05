package argocd

import (
	"fmt"
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/thelma/gitops"
	"sort"
	"strings"
)

type Values struct {
	Destinations []Destination `yaml:"destinations"`
}

type Destination struct {
	Server    string `yaml:"server"`
	Namespace string `yaml:"namespace"`
}

// Generate a set of values with a list of ArgoCD project destinations, in the form:
//
// destinations:
//   - server: https://<cluster api address>/
//     namespace: namespace1
//   - server: https://<cluster api address>/
//     namespace: namespace2
func GetDestinationValues(target gitops.Target) Values {
	destinations := destinationsForTarget(target)

	// Sort destinations so they always render in a consistent order
	sort.Slice(destinations, func(i, j int) bool {
		return destinations[i].compare(destinations[j]) < 0
	})

	return Values{Destinations: destinations}
}

func GetProjectName(target gitops.Target) string {
	switch t := target.(type) {
	case gitops.Environment:
		return fmt.Sprintf("terra-%s", t.Name())
	case gitops.Cluster:
		return fmt.Sprintf("cluster-%s", t.Name())
	default:
		panic(fmt.Errorf("error generating destination values file: unknown target type %s: %v", target.Type().String(), target))
	}
}

func destinationsForTarget(target gitops.Target) []Destination {
	switch t := target.(type) {
	case gitops.Environment:
		return destinationsForEnvironment(t)
	case gitops.Cluster:
		return destinationsForCluster(t)
	default:
		panic(fmt.Errorf("error generating destination values file: unknown target type %s: %v", target.Type().String(), target))
	}
}

func destinationsForCluster(cluster gitops.Cluster) []Destination {
	return []Destination{
		{
			Server:    cluster.Address(),
			Namespace: "*", // Cluster releases can deploy to any namespace
		},
	}
}

func destinationsForEnvironment(environment gitops.Environment) []Destination {
	clusterAddresses := make(map[string]bool)
	for _, release := range environment.Releases() {
		clusterAddresses[release.ClusterAddress()] = true
	}

	var destinations []Destination
	for address := range clusterAddresses {
		destinations = append(destinations, Destination{
			Server:    address,
			Namespace: environment.Namespace(),
		})
	}

	return destinations
}

// Return -1 if d < other, 0 if d == other, +1 if d > other
func (d Destination) compare(other Destination) int {
	byServer := strings.Compare(d.Server, other.Server)
	if byServer != 0 {
		return byServer
	}

	byNamespace := strings.Compare(d.Namespace, other.Namespace)
	return byNamespace
}
