// package services
//
// import (
//
//	"context"
//	"fmt"
//	"github.com/bndr/gojenkins"
//
// )
//
// var Jenkins *gojenkins.Jenkins
//
// func Login() {
//
//		ctx := context.Background()
//		Jenkins := gojenkins.CreateJenkins(nil, GetConf().Host, GetConf().User, GetConf().Password)
//		_, err := Jenkins.Init(ctx)
//
//		fmt.Println(GetConf().Host, GetConf().User, GetConf().Password)
//
//		if err != nil {
//			panic("Something Went Wrong")
//		}
//	}
package services

import (
	"context"
	"fmt"
	"github.com/bndr/gojenkins"
)

var Jenkins *gojenkins.Jenkins

func Login() {
	ctx := context.Background()
	Jenkins = gojenkins.CreateJenkins(nil, GetConf().Host, GetConf().User, GetConf().Password)
	_, err := Jenkins.Init(ctx)

	if err != nil {
		fmt.Println("111111111")
		panic("Something Went Wrong")
	}
}
