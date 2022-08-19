package main

import (
	"pixiu/routes"
	"pixiu/services"
)

func main() {
	services.JenkinsLogin()
	//services.AddViewJob()
	//services.RunJob()
	//services.CreateJob()
	r := routes.RoutesRunJob()
	//b := routes.RoutesCreateJob()
	// 3.监听端口，默认在8080
	// Run("里面不指定端口号默认为8080")
	r.Run(":8000")
	//b.Run(":8000")
}
