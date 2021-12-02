package gitops

// VersionSet is an enum type representing a version set defined in terra-helmfile
type VersionSet int

const (
	Dev VersionSet = iota
	Alpha
	Staging
	Prod
)

func VersionSets() []VersionSet {
	return []VersionSet{
		Dev,
		Alpha,
		Staging,
		Prod,
	}
}

func (s VersionSet) String() string {
	switch s {
	case Dev:
		return "dev"
	case Alpha:
		return "alpha"
	case Staging:
		return "staging"
	case Prod:
		return "prod"
	}
	return "unknown"
}
