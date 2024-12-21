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

package helm

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"k8s.io/klog/v2"

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
	Create(ctx context.Context, repo *types.CreateRepository) error
	Delete(ctx context.Context, id int64) error
	Get(ctx context.Context, id int64) (*model.Repository, error)
	List(ctx context.Context) ([]*model.Repository, error)
	Update(ctx context.Context, id int64, update *types.UpdateRepository) error

	GetChartsById(ctx context.Context, id int64) (*model.ChartIndex, error)
	GetChartsByURL(ctx context.Context, repoURL string) (*model.ChartIndex, error)
	GetChartValues(ctx context.Context, chart, version string) (string, error)
}

type Repository struct {
	settings     *cli.EnvSettings
	actionConfig *action.Configuration
	factory      db.ShareDaoFactory
}

func NewRepository(f db.ShareDaoFactory) *Repository {
	settings := cli.New()
	actionConfig := new(action.Configuration)
	actionConfig.Init(settings.RESTClientGetter(), settings.Namespace(), "secrets", klog.Infof)
	return &Repository{factory: f, settings: settings, actionConfig: actionConfig}
}

var _ RepositoryInterface = &Repository{}

func (r *Repository) Create(ctx context.Context, repo *types.CreateRepository) error {

	repoModel := &model.Repository{
		Name: repo.Name,
		URL:  repo.URL,
	}
	if res, _ := r.GetByName(ctx, repoModel.Name); res != nil {
		return fmt.Errorf("repository %s already exists", repoModel.Name)
	}

	_, err := r.factory.Repository().Create(ctx, repoModel)
	return err
}

func (r *Repository) Delete(ctx context.Context, id int64) error {
	return r.factory.Repository().Delete(ctx, id)
}

func (r *Repository) Get(ctx context.Context, id int64) (*model.Repository, error) {
	return r.factory.Repository().Get(ctx, id)
}

func (r *Repository) GetByName(ctx context.Context, name string) (*model.Repository, error) {
	return r.factory.Repository().GetByName(ctx, name)
}

func (r *Repository) List(ctx context.Context) ([]*model.Repository, error) {
	return r.factory.Repository().List(ctx)
}

func (r *Repository) Update(ctx context.Context, id int64, update *types.UpdateRepository) error {
	updates := map[string]interface{}{
		"name":     update.Name,
		"url":      update.URL,
		"username": update.Username,
		"password": update.Password,
	}
	return r.factory.Repository().Update(ctx, id, *update.ResourceVersion, updates)
}

func (r *Repository) GetChartsById(ctx context.Context, id int64) (*model.ChartIndex, error) {
	repository, err := r.Get(ctx, id)
	if err != nil {
		return nil, err
	}

	entry := &repo.Entry{
		Name:     repository.Name,
		URL:      repository.URL,
		Username: repository.Username,
		Password: repository.Password,
	}
	return r.fetch(ctx, entry)
}

func (r *Repository) GetChartsByURL(ctx context.Context, repoURL string) (*model.ChartIndex, error) {
	entry := &repo.Entry{
		URL: repoURL,
	}
	return r.fetch(ctx, entry)
}

func (r *Repository) GetChartValues(_ context.Context, chart, version string) (string, error) {
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
