package versions

// Set is an enum type representing a version set defined in terra-helmfile
type Set int

const (
	Dev Set = iota
	Alpha
	Staging
	Prod
)

func (s Set) String() string {
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
