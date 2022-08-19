package services

import (
	"context"
	"fmt"
	"github.com/bndr/gojenkins"
	"pixiu/variable"
	"time"
)

func CreateJob() {
	ctx := context.Background()
	configString := `<?xml version='1.0' encoding='UTF-8'?>
<project>
  <actions/>
  <description></description>
  <keepDependencies>false</keepDependencies>
  <properties/>
  <scm class="hudson.scm.NullSCM"/>
  <canRoam>true</canRoam>
  <disabled>false</disabled>
  <blockBuildWhenDownstreamBuilding>false</blockBuildWhenDownstreamBuilding>
  <blockBuildWhenUpstreamBuilding>false</blockBuildWhenUpstreamBuilding>
  <triggers class="vector"/>
  <concurrentBuild>false</concurrentBuild>
  <builders/>
  <publishers/>
  <buildWrappers/>
</project>`
	CreateJob, err := Jenkins.CreateJob(ctx, configString, "123213213213")
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println(CreateJob.Base)
}

func AddViewJob() {
	ctx := context.Background()
	view, err := Jenkins.CreateView(ctx, variable.CreateJobViewName, gojenkins.LIST_VIEW)

	if err != nil {
		fmt.Println(err)
	}

	status, err := view.AddJob(ctx, "tt")

	var nil bool
	if status != nil {
		fmt.Println("Job has been added to view")

	}

	//create, _ := Jenkins.CreateJob(ctx, "", "CreateJobTest")

	//fmt.Println(create)
}

func RunJob() {
	ctx := context.Background()
	queueid, err := Jenkins.BuildJob(ctx, variable.RunJobName, nil)
	if err != nil {
		fmt.Println(err)
	}
	build, err := Jenkins.GetBuildFromQueueID(ctx, queueid)
	if err != nil {
		fmt.Println(err)
	}

	// Wait for build to finish
	for build.IsRunning(ctx) {
		time.Sleep(5000 * time.Millisecond)
		build.Poll(ctx)
	}

	fmt.Printf("build number %d with result: %v\n", build.GetBuildNumber(), build.GetResult())
}

func DeleteJob() {
	ctx := context.Background()
	DeleteJob, err := Jenkins.DeleteJob(ctx, variable.DeleteJobName)
	if err != nil {
		fmt.Println(err)
	}
	fmt.Printf("job 已经删除%v", DeleteJob)
}
