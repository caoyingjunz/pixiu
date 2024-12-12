package cluster

import (
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/repo"
)

type IRepositories interface {
	List() ([]*repo.Entry, error)
}

type Repositories struct {
	settings *cli.EnvSettings
}

func newRepositories(settings *cli.EnvSettings) *Repositories {
	return &Repositories{settings: settings}
}

var _ IRepositories = &Repositories{}

func (r *Repositories) List() ([]*repo.Entry, error) {
	f, err := r.load(r.settings.RepositoryConfig)
	if err != nil {
		return nil, err
	}
	return f.Repositories, nil
}

func (r *Repositories) load(c string) (*repo.File, error) {
	return repo.LoadFile(c)
}
