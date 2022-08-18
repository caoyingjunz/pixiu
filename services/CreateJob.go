package services

import (
	"context"
	"fmt"
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
	CreateJob, err := Jenkins.CreateJob(ctx, configString, "6666")
	if err != nil {
		panic(err)
	}
	fmt.Println(CreateJob.Base)
}
