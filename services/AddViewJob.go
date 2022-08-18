package services

import (
	"context"
	"fmt"
	"github.com/bndr/gojenkins"
)

func AddViewJob() {
	ctx := context.Background()
	view, err := Jenkins.CreateView(ctx, "CreateJob-view", gojenkins.LIST_VIEW)

	if err != nil {
		panic(err)
	}

	status, err := view.AddJob(ctx, "213123")

	var nil bool
	if status != nil {
		fmt.Println("Job has been added to view")
	}

	//create, _ := Jenkins.CreateJob(ctx, "", "CreateJobTest")

	//fmt.Println(create)
}
