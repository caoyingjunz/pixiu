package cluster

import (
	"context"
	"fmt"
	"io"

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/downloader"
	"helm.sh/helm/v3/pkg/getter"
	"helm.sh/helm/v3/pkg/registry"
	"helm.sh/helm/v3/pkg/release"
	"k8s.io/klog/v2"

	"github.com/caoyingjunz/pixiu/pkg/types"
)

type IReleases interface {
	GetRelease(ctx context.Context, name string) (*release.Release, error)
	ListRelease(ctx context.Context) ([]*release.Release, error)
	InstallRelease(ctx context.Context, form *types.ReleaseForm) (*release.Release, error)
	UninstallRelease(ctx context.Context, name string) (*release.UninstallReleaseResponse, error)
	UpgradeRelease(ctx context.Context, form *types.ReleaseForm) (*release.Release, error)
	GetReleaseHistory(ctx context.Context, name string) ([]*release.Release, error)
	RollbackRelease(ctx context.Context, name string, toVersion int) error
}

type Releases struct {
	settings     *cli.EnvSettings
	actionConfig *action.Configuration
}

func newReleases(actionCofnig *action.Configuration, settings *cli.EnvSettings) *Releases {
	return &Releases{
		actionConfig: actionCofnig,
		settings:     settings,
	}
}

var _ IReleases = &Releases{}

func (r *Releases) GetRelease(ctx context.Context, name string) (*release.Release, error) {
	client := action.NewGet(r.actionConfig)
	return client.Run(name)
}

func (r *Releases) ListRelease(ctx context.Context) ([]*release.Release, error) {
	client := action.NewList(r.actionConfig)
	return client.Run()
}

// install release
func (r *Releases) InstallRelease(ctx context.Context, form *types.ReleaseForm) (*release.Release, error) {
	client := action.NewInstall(r.actionConfig)
	client.ReleaseName = form.Name
	client.Namespace = r.settings.Namespace()
	client.Version = form.Version

	client.DryRun = form.Preview
	if client.DryRun {
		client.Description = "server"
	}
	chrt, err := r.locateChart(client.ChartPathOptions, form.Chart, r.settings)
	if err != nil {
		return nil, err
	}
	out, err := client.Run(chrt, form.Values)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (r *Releases) UninstallRelease(ctx context.Context, name string) (*release.UninstallReleaseResponse, error) {
	client := action.NewUninstall(r.actionConfig)
	return client.Run(name)
}

// upgrade release
func (r *Releases) UpgradeRelease(ctx context.Context, form *types.ReleaseForm) (*release.Release, error) {
	client := action.NewUpgrade(r.actionConfig)
	client.Namespace = r.settings.Namespace()
	client.DryRun = form.Preview
	if client.DryRun {
		client.Description = "server"
	}

	chrt, err := r.locateChart(client.ChartPathOptions, form.Chart, r.settings)
	if err != nil {
		return nil, err
	}

	out, err := client.Run(form.Name, chrt, form.Values)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (r *Releases) GetReleaseHistory(ctx context.Context, name string) ([]*release.Release, error) {
	client := action.NewHistory(r.actionConfig)
	return client.Run(name)
}

func (r *Releases) RollbackRelease(ctx context.Context, name string, toVersion int) error {
	klog.Error("version: ", toVersion)
	_, err := r.GetRelease(ctx, name)
	if err != nil {
		return err
	}

	client := action.NewRollback(r.actionConfig)
	client.Version = toVersion
	return client.Run(name)
}

func (r *Releases) locateChart(pathOpts action.ChartPathOptions, chart string, settings *cli.EnvSettings) (*chart.Chart, error) {
	// from cmd/helm/install.go and cmd/helm/upgrade.go
	cp, err := pathOpts.LocateChart(chart, settings)
	if err != nil {
		return nil, err
	}

	p := getter.All(settings)

	// Check chart dependencies to make sure all are present in /charts
	chartRequested, err := loader.Load(cp)
	if err != nil {
		return nil, err
	}

	if err := checkIfInstallable(chartRequested); err != nil {
		return nil, err
	}

	registryClient, err := registry.NewClient(
		registry.ClientOptDebug(false),
		//registry.ClientOptWriter(out),
		registry.ClientOptCredentialsFile(settings.RegistryConfig),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to crete helm config object %v", err)
	}

	if req := chartRequested.Metadata.Dependencies; req != nil {
		// If CheckDependencies returns an error, we have unfulfilled dependencies.
		// As of Helm 2.4.0, this is treated as a stopping condition:
		// https://github.com/helm/helm/issues/2209
		if err := action.CheckDependencies(chartRequested, req); err != nil {
			err = fmt.Errorf("an error occurred while checking for chart dependencies. You may need to run `helm dependency build` to fetch missing dependencies: %v", err)
			if true { // client.DependencyUpdate
				man := &downloader.Manager{
					Out:              io.Discard,
					ChartPath:        cp,
					Keyring:          pathOpts.Keyring,
					SkipUpdate:       false,
					Getters:          p,
					RepositoryConfig: settings.RepositoryConfig,
					RepositoryCache:  settings.RepositoryCache,
					Debug:            settings.Debug,
					RegistryClient:   registryClient, // added on top of Helm code
				}
				if err := man.Update(); err != nil {
					return nil, err
				}
				// Reload the chart with the updated Chart.lock file.
				if chartRequested, err = loader.Load(cp); err != nil {
					return nil, fmt.Errorf("failed reloading chart after repo update : %v", err)
				}
			} else {
				return nil, err
			}
		}
	}

	return chartRequested, nil
}

func checkIfInstallable(ch *chart.Chart) error {
	switch ch.Metadata.Type {
	case "", "application":
		return nil
	}
	return fmt.Errorf("%s charts are not installable", ch.Metadata.Type)
}
