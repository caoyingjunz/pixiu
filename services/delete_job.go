package services

import (
	"context"
	"fmt"
	"pixiu/variable"
)

func DeleteJob() {
	ctx := context.Background()
	DeleteJob, err := Jenkins.DeleteJob(ctx, variable.DeleteJobName)
	if err != nil {
		panic(err)
	}
	fmt.Printf("job 已经删除%v", DeleteJob)
}
