/*
Copyright 2021 The Pixiu Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package cluster

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/getter"
	"helm.sh/helm/v3/pkg/repo"
	"k8s.io/apimachinery/pkg/util/yaml"

	"github.com/caoyingjunz/pixiu/pkg/db"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/types"
)

type RepositoryGetter interface {
	Repository() RepositoryInterface
}

type RepositoryInterface interface {
	Create(ctx context.Context, repo *types.RepoForm) error
	Delete(ctx context.Context, id int64) error
	Get(ctx context.Context, id int64) (*model.Repository, error)
	List(ctx context.Context) ([]*model.Repository, error)
	Update(ctx context.Context, id int64, update *types.RepoUpdateForm) error

	GetRepoChartsById(ctx context.Context, id int64) (*model.ChartIndex, error)
	GetRepoChartsByURL(ctx context.Context, repoURL string) (*model.ChartIndex, error)
	GetRepoChartValues(ctx context.Context, chart, version string) (string, error)
}

type Repository struct {
	cluster      string
	settings     *cli.EnvSettings
	actionConfig *action.Configuration
	factory      db.ShareDaoFactory
}

func newRepository(cluster string, settings *cli.EnvSettings, actionConfig *action.Configuration, f db.ShareDaoFactory) *Repository {
	return &Repository{cluster: cluster, settings: settings, actionConfig: actionConfig, factory: f}
}

var _ RepositoryInterface = &Repository{}

func (r *Repository) Create(ctx context.Context, repo *types.RepoForm) error {
	repoModel := &model.Repository{
		Cluster:               r.cluster,
		Name:                  repo.Name,
		URL:                   repo.URL,
		Username:              repo.Username,
		Password:              repo.Password,
		CertFile:              repo.CertFile,
		KeyFile:               repo.KeyFile,
		CAFile:                repo.CAFile,
		InsecureSkipTLSverify: repo.InsecureSkipTLSverify,
		PassCredentialsAll:    repo.PassCredentialsAll,
	}
	if res, _ := r.GetByName(ctx, repoModel.Name); res != nil {
		return fmt.Errorf("repository %s already exists", repoModel.Name)
	}

	_, err := r.factory.Repository().Create(ctx, repoModel)
	return err
}

func (r *Repository) Delete(ctx context.Context, id int64) error {
	return r.factory.Repository().Delete(ctx, r.cluster, id)
}

func (r *Repository) Get(ctx context.Context, id int64) (*model.Repository, error) {
	return r.factory.Repository().Get(ctx, r.cluster, id)
}

func (r *Repository) GetByName(ctx context.Context, name string) (*model.Repository, error) {
	return r.factory.Repository().GetByName(ctx, r.cluster, name)
}

func (r *Repository) List(ctx context.Context) ([]*model.Repository, error) {
	return r.factory.Repository().List(ctx, r.cluster)
}

func (r *Repository) Update(ctx context.Context, id int64, update *types.RepoUpdateForm) error {
	updates := map[string]interface{}{
		"name":                     update.Name,
		"url":                      update.URL,
		"username":                 update.Username,
		"password":                 update.Password,
		"cert_file":                update.CertFile,
		"key_file":                 update.KeyFile,
		"ca_file":                  update.CAFile,
		"insecure_skip_tls_verify": update.InsecureSkipTLSverify,
		"pass_credentials_all":     update.PassCredentialsAll,
	}
	return r.factory.Repository().Update(ctx, r.cluster, id, *update.ResourceVersion, updates)
}

func (r *Repository) GetRepoChartsById(ctx context.Context, id int64) (*model.ChartIndex, error) {
	repository, err := r.Get(ctx, id)
	if err != nil {
		return nil, err
	}

	entry := &repo.Entry{
		Name:                  repository.Name,
		URL:                   repository.URL,
		Username:              repository.Username,
		Password:              repository.Password,
		CertFile:              repository.CertFile,
		KeyFile:               repository.KeyFile,
		CAFile:                repository.CAFile,
		InsecureSkipTLSverify: repository.InsecureSkipTLSverify,
		PassCredentialsAll:    repository.PassCredentialsAll,
	}
	return r.fetch(ctx, entry)
}

func (r *Repository) GetRepoChartsByURL(ctx context.Context, repoURL string) (*model.ChartIndex, error) {
	entry := &repo.Entry{
		URL: repoURL,
	}
	return r.fetch(ctx, entry)
}

func (r *Repository) GetRepoChartValues(ctx context.Context, chart, version string) (string, error) {
	client := action.NewShowWithConfig(action.ShowValues, r.actionConfig)
	client.Version = version
	cp, err := client.ChartPathOptions.LocateChart(chart, r.settings)
	if err != nil {
		return "", err
	}

	out, err := client.Run(cp)
	if err != nil {
		return "", err
	}

	return out, nil
}

func (r *Repository) resolveReferenceURL(baseURL, refURL string) (string, error) {
	parsedRefURL, err := url.Parse(refURL)
	if err != nil {
		return "", fmt.Errorf("failed to parse %s as URL, %v", refURL, err)
	}

	if parsedRefURL.IsAbs() {
		return refURL, nil
	}

	parsedBaseURL, err := url.Parse(baseURL)
	if err != nil {
		return "", fmt.Errorf("failed to parse %s as URL, %v", baseURL, err)
	}

	parsedBaseURL.RawPath = strings.TrimSuffix(parsedBaseURL.RawPath, "/") + "/"
	parsedBaseURL.Path = strings.TrimSuffix(parsedBaseURL.Path, "/") + "/"

	resolvedURL := parsedBaseURL.ResolveReference(parsedRefURL)
	resolvedURL.RawQuery = parsedBaseURL.RawQuery
	return resolvedURL.String(), nil
}

func (r *Repository) fetch(_ context.Context, entry *repo.Entry) (*model.ChartIndex, error) {
	var charts model.ChartIndex

	rep, err := repo.NewChartRepository(entry, getter.All(r.settings))
	if err != nil {
		return nil, err
	}

	// download index
	if _, err = rep.DownloadIndexFile(); err != nil {
		return nil, err
	}

	indexURL, err := r.resolveReferenceURL(rep.Config.URL, "index.yaml")
	if err != nil {
		return nil, err
	}

	resp, err := rep.Client.Get(indexURL,
		getter.WithURL(rep.Config.URL),
		getter.WithInsecureSkipVerifyTLS(rep.Config.InsecureSkipTLSverify),
		getter.WithTLSClientConfig(rep.Config.CertFile, rep.Config.KeyFile, rep.Config.CAFile),
		getter.WithBasicAuth(rep.Config.Username, rep.Config.Password),
		getter.WithPassCredentialsAll(rep.Config.PassCredentialsAll),
	)
	if err != nil {
		return nil, err
	}

	err = yaml.Unmarshal(resp.Bytes(), &charts)
	if err != nil {
		return nil, err
	}
	return &charts, nil
}
