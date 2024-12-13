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

	"github.com/caoyingjunz/pixiu/pkg/db"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/types"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/getter"
	"helm.sh/helm/v3/pkg/repo"
	"k8s.io/apimachinery/pkg/util/yaml"
)

type RepoGetter interface {
	Repositories() Interface
}

type Interface interface {
	Create(ctx context.Context, repo *types.RepoForm) error
	Delete(ctx context.Context, id int64) error
	Get(ctx context.Context, id int64) (*model.Repositories, error)
	GetByName(ctx context.Context, name string) (*model.Repositories, error)
	List(ctx context.Context) ([]*model.Repositories, error)
	Update(ctx context.Context, id int64, update *types.RepoUpdateForm) error

	GetRepoChartsById(ctx context.Context, id int64) (*model.ChartIndex, error)
	GetRepoChartsByURL(ctx context.Context, repoURL string) (*model.ChartIndex, error)
}

type Repositories struct {
	settings *cli.EnvSettings
	factory  db.ShareDaoFactory
}

func NewRepositories(f db.ShareDaoFactory) *Repositories {
	return &Repositories{settings: cli.New(), factory: f}
}

var _ Interface = &Repositories{}

func (r *Repositories) Create(ctx context.Context, repo *types.RepoForm) error {
	repoModel := &model.Repositories{
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
	_, err := r.factory.Repository().Create(ctx, repoModel)
	return err
}

func (r *Repositories) Delete(ctx context.Context, id int64) error {
	return r.factory.Repository().Delete(ctx, id)
}

func (r *Repositories) Get(ctx context.Context, id int64) (*model.Repositories, error) {
	return r.factory.Repository().Get(ctx, id)
}

func (r *Repositories) GetByName(ctx context.Context, name string) (*model.Repositories, error) {
	return r.factory.Repository().GetByName(ctx, name)
}

func (r *Repositories) List(ctx context.Context) ([]*model.Repositories, error) {
	return r.factory.Repository().List(ctx)
}

func (r *Repositories) Update(ctx context.Context, id int64, update *types.RepoUpdateForm) error {
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
	return r.factory.Repository().Update(ctx, id, *update.ResourceVersion, updates)
}

func (r *Repositories) GetRepoChartsById(ctx context.Context, id int64) (*model.ChartIndex, error) {
	reposistory, err := r.Get(ctx, id)
	if err != nil {
		return nil, err
	}

	entry := &repo.Entry{
		Name:                  reposistory.Name,
		URL:                   reposistory.URL,
		Username:              reposistory.Username,
		Password:              reposistory.Password,
		CertFile:              reposistory.CertFile,
		KeyFile:               reposistory.KeyFile,
		CAFile:                reposistory.CAFile,
		InsecureSkipTLSverify: reposistory.InsecureSkipTLSverify,
		PassCredentialsAll:    reposistory.PassCredentialsAll,
	}
	return r.fetch(ctx, entry)
}

func (r *Repositories) GetRepoChartsByURL(ctx context.Context, repoURL string) (*model.ChartIndex, error) {
	entry := &repo.Entry{
		URL: repoURL,
	}
	return r.fetch(ctx, entry)
}

func (r *Repositories) resolveReferenceURL(baseURL, refURL string) (string, error) {
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

func (r *Repositories) fetch(ctx context.Context, entry *repo.Entry) (*model.ChartIndex, error) {
	var charts model.ChartIndex

	rep, err := repo.NewChartRepository(entry, getter.All(r.settings))
	if err != nil {
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
