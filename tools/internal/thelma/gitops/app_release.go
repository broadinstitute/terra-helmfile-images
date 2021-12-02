package gitops

type AppRelease interface {
	AppVersion() string
	Environment() Environment
	Release
}

type appRelease struct {
	appVersion string
	release
}

func (r *appRelease) AppVersion() string {
	return r.appVersion
}

func (r *appRelease) Environment() Environment {
	return r.target.(Environment)
}
