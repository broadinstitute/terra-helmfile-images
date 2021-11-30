package release

import (
	"fmt"
	"gopkg.in/yaml.v3"
)

// ReleaseType is an enum type referring to the two types of releases supported by terra-helmfile.
type ReleaseType int

const (
	AppType ReleaseType = iota
	ClusterType
)

// UnmarshalYAML is a custom unmarshaler so that the string "app" or "cluster" in a
// yaml file can be unmarshaled into a ReleaseType
func (r *ReleaseType) UnmarshalYAML(value *yaml.Node) error {
	switch value.Value {
	case "app":
		*r = AppType
		return nil
	case "cluster":
		*r = ClusterType
		return nil
	}

	return fmt.Errorf("unknown release type: %v", value.Value)
}

func (r ReleaseType) String() string {
	switch r {
	case AppType:
		return "app"
	case ClusterType:
		return "cluster"
	}
	return "unknown"
}
