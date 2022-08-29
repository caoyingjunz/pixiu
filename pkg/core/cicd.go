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

package core

import (
	"context"
	"fmt"
	"github.com/bndr/gojenkins"
	"time"

	"github.com/caoyingjunz/gopixiu/cmd/app/config"
	"github.com/caoyingjunz/gopixiu/pkg/db"
	"github.com/caoyingjunz/gopixiu/pkg/log"
	"github.com/caoyingjunz/gopixiu/pkg/types"
)

type CicdGetter interface {
	Cicd() CicdInterface
}

type CicdInterface interface {
	RunJob(ctx context.Context, name string) error
	CreateJob(ctx context.Context, name interface{}) error
	DeleteJob(ctx context.Context, name string) error
	AddViewJob(ctx context.Context, add_view_job string) error
	GetAllJobs(ctx context.Context) (res []string, err error)
	CopyJob(ctx context.Context, old_name string, new_name string) (res []string, err error)
	RenameJob(ctx context.Context, old_name string, new_name string) error
	SafeRestart(ctx context.Context, safeRestart string) error
}

type cicd struct {
	ComponentConfig config.Config
	app             *pixiu
	factory         db.ShareDaoFactory
	cicdDriver      *gojenkins.Jenkins
}

func newCicd(c *pixiu) CicdInterface {
	return &cicd{
		ComponentConfig: c.cfg,
		app:             c,
		factory:         c.factory,
		cicdDriver:      c.cicdDriver,
	}
}

func (c *cicd) RunJob(ctx context.Context, name string) error {
	queueid, err := c.cicdDriver.BuildJob(ctx, name, nil)
	if err != nil {
		return err
	}

	build, err := c.cicdDriver.GetBuildFromQueueID(ctx, queueid)
	if err != nil {
		fmt.Println(err)
	}

	// Wait for build to finish
	for build.IsRunning(ctx) {
		time.Sleep(5000 * time.Millisecond)
		build.Poll(ctx)
	}
	return nil
}

func (c *cicd) CreateJob(ctx context.Context, name interface{}) error {
	_, err := c.cicdDriver.CreateJob(ctx, types.JobStringConfig, name)
	if err != nil {
		log.Logger.Errorf("failed to create job %s: %v", name, err)
		return err
	}

	return nil
}

func (c *cicd) CopyJob(ctx context.Context, o_name string, n_name string) (res []string, err error) {
	_, err = c.cicdDriver.CopyJob(ctx, o_name, n_name)
	if err != nil {
		log.Logger.Errorf("failed to copy job %s: %v", o_name, err)
		return res, err
	}
	return res, nil
}

func (c *cicd) RenameJob(ctx context.Context, old_name string, new_name string) error {
	err := c.cicdDriver.RenameJob(ctx, old_name, new_name)
	if err != nil {
		log.Logger.Errorf("failed to rename job %s: %v", new_name, err)
		return nil
	}
	return nil
}

func (c *cicd) DeleteJob(ctx context.Context, name string) error {
	_, err := c.cicdDriver.DeleteJob(ctx, name)
	if err != nil {
		log.Logger.Errorf("failed to delete job %s: %v", name, err)
		return err
	}

	return nil
}

func (c *cicd) AddViewJob(ctx context.Context, add_view_job string) error {
	view, err := c.cicdDriver.CreateView(ctx, add_view_job, gojenkins.LIST_VIEW)
	if err != nil {
		log.Logger.Errorf("failed to create view %s: %v", add_view_job, err)
		return err
	}

	status, err := view.AddJob(ctx, "tt")
	if err != nil {
		log.Logger.Errorf("failed to add view %s: %v", add_view_job, err)
		return err
	}
	if !status {
		log.Logger.Errorf("failed to add view %s: %v", add_view_job, err)
		return err
	}

	return nil
}

func (c *cicd) GetAllJobs(ctx context.Context) (res []string, err error) {
	getalljobs, err := c.cicdDriver.GetAllJobs(ctx)
	if err != nil {
		log.Logger.Errorf("failed to getall job %s: %v", err)
		return nil, err
	}
	jobs := make([]string, 0)
	for _, value := range getalljobs {
		jobs = append(jobs, value.Base)
		//fmt.Println(value.Base)
	}
	return jobs, nil
}

func (c *cicd) SafeRestart(ctx context.Context, safeRestart string) error {
	err := c.cicdDriver.SafeRestart(ctx)
	if err != nil {
		log.Logger.Errorf("failed to safeRestart  %s: %v", safeRestart, err)
		return nil
	}
	return nil

}
