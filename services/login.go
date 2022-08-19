package services

import (
	"context"
	"fmt"
	"github.com/bndr/gojenkins"
)

var Jenkins *gojenkins.Jenkins

func JenkinsLogin() {
	ctx := context.Background()
	Jenkins = gojenkins.CreateJenkins(nil, GetConf().Host, GetConf().User, GetConf().Password)
	_, err := Jenkins.Init(ctx)

	if err != nil {
		fmt.Println(err)
	}
}
