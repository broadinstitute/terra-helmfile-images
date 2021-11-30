package release

type AppRelease struct {

}

func (r *AppRelease) Type() ReleaseType {
	return AppType
}
