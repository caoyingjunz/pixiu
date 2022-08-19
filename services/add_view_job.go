package services

import (
	"context"
	"fmt"
	"github.com/bndr/gojenkins"
	"pixiu/variable"
)

func AddViewJob() {
	ctx := context.Background()
	view, err := Jenkins.CreateView(ctx, variable.CreateJobViewName, gojenkins.LIST_VIEW)

	if err != nil {
		panic(err)
	}

	status, err := view.AddJob(ctx, "tt")

	var nil bool
	if status != nil {
		fmt.Println("Job has been added to view")
	}

	//create, _ := Jenkins.CreateJob(ctx, "", "CreateJobTest")

	//fmt.Println(create)
}
