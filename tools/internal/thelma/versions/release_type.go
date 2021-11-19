package versions

import (
	"fmt"
	"gopkg.in/yaml.v3"
)

// ReleaseType is an enum type referring to the two types of releases supported by terra-helmfile.
type ReleaseType int

const (
	AppRelease ReleaseType = iota
	ClusterRelease
)

// UnmarshalYAML is a custom unmarshaler so that the string "app" or "cluster" in a
// yaml file can be unmarshaled into a ReleaseType
func (r *ReleaseType) UnmarshalYAML(value *yaml.Node) error {
	switch value.Value {
	case "app":
		*r = AppRelease
		return nil
	case "cluster":
		*r = ClusterRelease
		return nil
	}

	return fmt.Errorf("unknown release type: %v", value.Value)
}

func (r ReleaseType) String() string {
	switch r {
	case AppRelease:
		return "app"
	case ClusterRelease:
		return "cluster"
	}
	return "unknown"
}
