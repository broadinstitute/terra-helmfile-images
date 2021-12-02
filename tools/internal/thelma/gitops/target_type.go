package gitops

import (
	"fmt"
	"gopkg.in/yaml.v3"
)

// TargetType is an enum type referring to the two types of targets supported by terra-helmfile.
type TargetType int

const (
	EnvironmentTargetType TargetType = iota
	ClusterTargetType
)

// Returns 0 if t == other, -1 if t < other, or +1 if t > other.
func (t TargetType) Compare(other TargetType) int {
	if t == other {
		return 0
	}
	if t == EnvironmentTargetType {
		return -1
	}
	return 1
}

// UnmarshalYAML is a custom unmarshaler so that the string "environment" or "cluster" in a
// yaml file can be unmarshaled into a TargetType
func (t *TargetType) UnmarshalYAML(value *yaml.Node) error {
	switch value.Value {
	case "environment":
		*t = EnvironmentTargetType
		return nil
	case "cluster":
		*t = ClusterTargetType
		return nil
	}

	return fmt.Errorf("unknown release type: %v", value.Value)
}

func (t TargetType) String() string {
	switch t {
	case EnvironmentTargetType:
		return "environment"
	case ClusterTargetType:
		return "cluster"
	}
	return "unknown"
}
