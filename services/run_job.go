package services

import (
	"context"
	"fmt"
	"pixiu/variable"
	"time"
)

func RunJob() {
	ctx := context.Background()
	queueid, err := Jenkins.BuildJob(ctx, variable.RunJobName, nil)
	if err != nil {
		panic(err)
	}
	build, err := Jenkins.GetBuildFromQueueID(ctx, queueid)
	if err != nil {
		panic(err)
	}

	// Wait for build to finish
	for build.IsRunning(ctx) {
		time.Sleep(5000 * time.Millisecond)
		build.Poll(ctx)
	}
	fmt.Printf("build number %d with result: %v\n", build.GetBuildNumber(), build.GetResult())
}
