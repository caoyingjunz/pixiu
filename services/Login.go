package services

import (
	"context"
	"fmt"
	"github.com/bndr/gojenkins"
)

var Jenkins *gojenkins.Jenkins

func Login() {
	ctx := context.Background()
	Jenkins = gojenkins.CreateJenkins(nil, "http://8.140.165.201:9090/", "adminops", "adminops@321")
	_, err := Jenkins.Init(ctx)

	if err != nil {
		fmt.Println("111111111")
		panic("Something Went Wrong")
	}
}
