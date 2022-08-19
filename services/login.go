package services

import (
	"context"
	"github.com/bndr/gojenkins"
)

var Jenkins *gojenkins.Jenkins

func JenkinsLogin() {
	ctx := context.Background()
	Jenkins = gojenkins.CreateJenkins(nil, GetConf().Host, GetConf().User, GetConf().Password)
	_, err := Jenkins.Init(ctx)

	if err != nil {
		panic("Something Went Wrong")
	}
}
