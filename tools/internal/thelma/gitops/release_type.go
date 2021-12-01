package gitops

import (
	"fmt"
	"gopkg.in/yaml.v3"
)

// ReleaseType is an enum type referring to the two types of releases supported by terra-helmfile.
type ReleaseType int

const (
	AppReleaseType ReleaseType = iota
	ClusterReleaseType
)

func ReleaseTypes() []ReleaseType {
	return []ReleaseType{AppReleaseType, ClusterReleaseType}
}

// Returns 0 if r == other, -1 if r < other, or +1 if r > other.
func (r ReleaseType) Compare(other ReleaseType) int {
	if r == other {
		return 0
	}
	if r == AppReleaseType {
		return -1
	}
	return 1
}

// UnmarshalYAML is a custom unmarshaler so that the string "app" or "cluster" in a
// yaml file can be unmarshaled into a ReleaseType
func (r *ReleaseType) UnmarshalYAML(value *yaml.Node) error {
	switch value.Value {
	case "app":
		*r = AppReleaseType
		return nil
	case "cluster":
		*r = ClusterReleaseType
		return nil
	}

	return fmt.Errorf("unknown release type: %v", value.Value)
}

func (r ReleaseType) String() string {
	switch r {
	case AppReleaseType:
		return "app"
	case ClusterReleaseType:
		return "cluster"
	}
	return "unknown"
}
