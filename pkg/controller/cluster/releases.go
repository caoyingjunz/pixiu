package cluster

import (
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/release"
)

type IReleases interface {
	ListRelease() ([]*release.Release, error)
}

type Releases struct {
	settings     *cli.EnvSettings
	actionCofnig *action.Configuration
}

func newReleases(actionCofnig *action.Configuration, settings *cli.EnvSettings) *Releases {
	return &Releases{
		actionCofnig: actionCofnig,
		settings:     settings,
	}
}

var _ IReleases = &Releases{}

func (r *Releases) ListRelease() ([]*release.Release, error) {
	client := action.NewList(r.actionCofnig)
	return client.Run()
}
