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
	"bytes"
	"context"
	"html/template"
	"strings"
	"time"

	"github.com/bndr/gojenkins"

	"github.com/caoyingjunz/gopixiu/pkg/types"
	types2 "github.com/caoyingjunz/gopixiu/api/types"
	"github.com/caoyingjunz/gopixiu/cmd/app/config"
	"github.com/caoyingjunz/gopixiu/pkg/db"
	"github.com/caoyingjunz/gopixiu/pkg/log"
)

type CicdGetter interface {
	Cicd() CicdInterface
}

type CicdInterface interface {
	RunJob(ctx context.Context, name string) error
	CreateJob(ctx context.Context, cicd types2.Cicd) error
	DeleteJob(ctx context.Context, name string) error
	DeleteViewJob(ctx context.Context, name string, viewname string) (bool, error)
	AddViewJob(ctx context.Context, addViewJob string, name string) error
	GetAllJobs(ctx context.Context) (res []string, err error)
	GetAllViews(ctx context.Context) (views []string, err error)
	Details(ctx context.Context, name string) *gojenkins.JobResponse
	GetAllNodes(ctx context.Context) (nodes []string, err error)
	DeleteNode(ctx context.Context, name string) error
	CopyJob(ctx context.Context, oldName string, newName string) (res []string, err error)
	RenameJob(ctx context.Context, oldName string, newName string) error
	Restart(ctx context.Context) error
	Disable(ctx context.Context, name string) (bool, error)
	Enable(ctx context.Context, name string) (bool, error)
	Stop(ctx context.Context, name string) (bool, error)
	Config(ctx context.Context, name string) (config string, err error)
	UpdateConfig(ctx context.Context, name string) error
	History(ctx context.Context, name string) ([]*gojenkins.History, error)
	GetLastFailedBuild(ctx context.Context, name string) (gojenkins.JobBuild, error)
	GetLastSuccessfulBuild(ctx context.Context, name string) (getLastSuccessfulBuild gojenkins.JobBuild, err error)
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
		return err
	}
	// Wait for build to finish
	for build.IsRunning(ctx) {
		time.Sleep(5000 * time.Millisecond)
		build.Poll(ctx)
	}
	return nil
}

func (c *cicd) CreateJob(ctx context.Context, cicd types2.Cicd) error {
	buf := new(bytes.Buffer)
	if cicd.Type == "PipLineStyle" {
		temp, _ := template.New("test").Parse(types.PipLineStyleConfig)
		temp.Execute(buf, cicd.Git)
		//处理转义
		p := strings.Replace(buf.String(), "&lt;", "<", -1)
		if _, err := c.cicdDriver.CreateJob(ctx, p, cicd.Name); err != nil {
			log.Logger.Errorf("failed to create job %s: %v", cicd.Name, err)
			return err
		}
	} else {
		temp, _ := template.New("test").Parse(types.FreeStyleConfig)
		temp.Execute(buf, cicd.Git)
		p := strings.Replace(buf.String(), "&lt;", "<", -1)
		if _, err := c.cicdDriver.CreateJob(ctx, p, cicd.Name); err != nil {
			log.Logger.Errorf("failed to create job %s: %v", cicd.Name, err)
			return err
		}
	}
	return nil
}

func (c *cicd) CopyJob(ctx context.Context, oldName string, newName string) (res []string, err error) {
	if _, err = c.cicdDriver.CopyJob(ctx, oldName, newName); err != nil {
		log.Logger.Errorf("failed to copy job %s: %v", oldName, err)
		return res, err
	}
	return res, nil
}

func (c *cicd) RenameJob(ctx context.Context, oldName string, newName string) error {
	if err := c.cicdDriver.RenameJob(ctx, oldName, newName); err != nil {
		log.Logger.Errorf("failed to rename job %s: %v", newName, err)
		return nil
	}
	return nil
}

func (c *cicd) DeleteJob(ctx context.Context, name string) error {
	if _, err := c.cicdDriver.DeleteJob(ctx, name); err != nil {
		log.Logger.Errorf("failed to delete job %s: %v", name, err)
		return err
	}
	return nil
}

func (c *cicd) DeleteViewJob(ctx context.Context, name string, viewname string) (bool, error) {
	view := &gojenkins.View{Jenkins: c.cicdDriver}
	view, err := c.cicdDriver.GetView(ctx, viewname)
	if err != nil {
		log.Logger.Errorf("failed to found view%s: %v", viewname, err)
		return false, err
	}
	if _, err = view.DeleteJob(ctx, name); err != nil {
		log.Logger.Errorf("failed to delete viewjob %s: %v", name, err)
		return false, err
	}

	return true, nil
}

func (c *cicd) DeleteNode(ctx context.Context, name string) error {
	if _, err := c.cicdDriver.DeleteJob(ctx, name); err != nil {
		log.Logger.Errorf("failed to delete Node %s: %v", name, err)
		return err
	}
	return nil
}

func (c *cicd) AddViewJob(ctx context.Context, addViewJob string, name string) error {
	view, err := c.cicdDriver.CreateView(ctx, addViewJob, gojenkins.LIST_VIEW)
	if err != nil {
		log.Logger.Errorf("failed to create view %s: %v", addViewJob, err)
		return err
	}
	status, err := view.AddJob(ctx, name)
	if err != nil {
		log.Logger.Errorf("failed to add view %s: %v", addViewJob, err)
		return err
	}
	if !status {
		log.Logger.Errorf("failed to add view %s: %v", addViewJob, err)
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
	}
	return jobs, nil
}

func (c *cicd) GetAllViews(ctx context.Context) (views []string, err error) {
	getallview, err := c.cicdDriver.GetAllViews(ctx)
	if err != nil {
		log.Logger.Errorf("failed to getall views %s: %v", err)
		return nil, err
	}
	view := make([]string, 0)
	for _, value := range getallview {
		view = append(view, value.Base)
	}
	return view, nil
}

func (c *cicd) GetAllNodes(ctx context.Context) (nodes []string, err error) {
	getallnodes, err := c.cicdDriver.GetAllNodes(ctx)
	if err != nil {
		log.Logger.Errorf("failed to getall nodes %s: %v", err)
		return nil, err
	}
	nodes = make([]string, 0)
	for _, value := range getallnodes {
		nodes = append(nodes, value.Base)
	}
	return nodes, nil
}

func (c *cicd) Restart(ctx context.Context) error {
	if err := c.cicdDriver.SafeRestart(ctx); err != nil {
		log.Logger.Errorf("failed to Restart %v", err)
		return nil
	}
	return nil
}

func (c *cicd) Disable(ctx context.Context, name string) (bool, error) {
	job, err := c.cicdDriver.GetJob(ctx, name)
	if err != nil {
		log.Logger.Errorf("failed to Disable %v", err)
		return false, nil
	}
	if _, err = job.Disable(ctx); err != nil {
		log.Logger.Errorf("failed to Disable %v", err)
		return false, nil
	}
	return true, nil
}

func (c *cicd) Enable(ctx context.Context, name string) (bool, error) {
	job, err := c.cicdDriver.GetJob(ctx, name)
	if err != nil {
		log.Logger.Errorf("failed to get job %v", err)
		return false, nil
	}
	if _, err = job.Enable(ctx); err != nil {
		log.Logger.Errorf("failed to Enable %v", err)
		return false, nil
	}
	return true, nil
}

func (c *cicd) Stop(ctx context.Context, name string) (bool, error) {
	build := &gojenkins.Build{Jenkins: c.cicdDriver}
	var err error
	if build.Job, err = c.cicdDriver.GetJob(ctx, name); err != nil {
		return false, err
	}
	if _, err = build.Stop(ctx); err != nil {
		return false, err
	}

	return true, nil
}

func (c *cicd) Details(ctx context.Context, name string) *gojenkins.JobResponse {
	var err error
	job, err := c.cicdDriver.GetJob(ctx, name)
	if err != nil {
		log.Logger.Errorf("failed to Enable %v", err)
		return nil
	}
	details := job.GetDetails()
	return details
}

func (c *cicd) Config(ctx context.Context, name string) (config string, err error) {
	job, err := c.cicdDriver.GetJob(ctx, name)
	if err != nil {
		log.Logger.Errorf("failed to get job %v", err)
		return config, nil
	}
	config, err = job.GetConfig(ctx)
	if err != nil {
		log.Logger.Errorf("failed to Enable %v", err)
		return config, nil
	}
	return config, nil
}

func (c *cicd) UpdateConfig(ctx context.Context, name string) error {
	job, err := c.cicdDriver.GetJob(ctx, name)
	if err != nil {
		log.Logger.Errorf("failed to get job %v", err)
		return nil
	}
	if err = job.UpdateConfig(ctx, "dsfd"); err != nil {
		log.Logger.Errorf("failed to UpdateConfig %v", err)
		return nil
	}
	return nil
}

func (c *cicd) GetLastFailedBuild(ctx context.Context, name string) (getLastFailedBuild1 gojenkins.JobBuild, err error) {
	job, err := c.cicdDriver.GetJob(ctx, name)
	if err != nil {
		log.Logger.Errorf("failed to  jobs %s: %v", err)
		return getLastFailedBuild1, err
	}
	failedBuild := job.Raw.LastFailedBuild
	if err != nil {
		log.Logger.Errorf("failed to GetLastFailedBuild %v", err)
		return failedBuild, nil
	}
	return failedBuild, nil
}

func (c *cicd) GetLastSuccessfulBuild(ctx context.Context, name string) (getLastSuccessfulBuild gojenkins.JobBuild, err error) {
	job, err := c.cicdDriver.GetJob(ctx, name)
	if err != nil {
		log.Logger.Errorf("failed to  jobs %s: %v", err)
		return getLastSuccessfulBuild, err
	}
	successBuild := job.Raw.LastSuccessfulBuild
	if err != nil {
		log.Logger.Errorf("failed to GetLastSuccessfulBuild %v", err)
		return successBuild, nil
	}
	return successBuild, nil
}

func (c *cicd) History(ctx context.Context, name string) ([]*gojenkins.History, error) {
	job, err := c.cicdDriver.GetJob(ctx, name)
	if err != nil {
		log.Logger.Errorf("failed to  jobs %s: %v", err)
		return nil, err
	}
	history, err := job.History(ctx)
	if err != nil {
		log.Logger.Errorf("failed to GetLastSuccessfulBuild %v", err)
		return nil, err
	}
	return history, nil
}
