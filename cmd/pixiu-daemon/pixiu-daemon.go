/*
Copyright 2021 The Pixiu Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"flag"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/rest"
	"time"

	"k8s.io/klog/v2"

	"github.com/caoyingjunz/pixiu/cmd/pixiu-controller-manager/app"
	isclientset "github.com/caoyingjunz/pixiu/pkg/client/clientset/versioned"
	pClientset "github.com/caoyingjunz/pixiu/pkg/client/clientset/versioned"
	informer "github.com/caoyingjunz/pixiu/pkg/client/informers/externalversions"
	"github.com/caoyingjunz/pixiu/pkg/controller"
	"github.com/caoyingjunz/pixiu/pkg/controller/imageset"
	"github.com/caoyingjunz/pixiu/pkg/signals"
	kubeinformers "k8s.io/client-go/informers"
	clientset "k8s.io/client-go/kubernetes"
)

const (
	HealthzHost = "127.0.0.1"
	HealthzPort = "10258"
)

type ControllerContext struct {
	// ClientBuilder will provide a client for this controller to use
	ClientBuilder controller.ControllerClientBuilder
	// PixiuClientBuilder will provide a client for pixiu controller to use
	PixiuClientBuilder func(c *rest.Config) *pClientset.Clientset

	// InformerFactory gives access to informers for the controller.
	InformerFactory informers.SharedInformerFactory

	ObjectOrMetadataInformerFactory controller.InformerFactory


	// AvailableResources is a map listing currently available resources for pixiu
	AvailableResources map[schema.GroupVersionResource]bool

	// Stop is the stop channel
	Stop <-chan struct{}

	// KubeConfig is the given config to cluster
	KubeConfig *rest.Config

	// ResyncPeriod generates a duration each time it is invoked; this is so that
	// multiple controllers don't get into lock-step and all hammer the apiserver
	// with list requests simultaneously.
	ResyncPeriod func() time.Duration
}

func main() {
	klog.InitFlags(nil)
	flag.Parse()

	// set up signals so we handle the first shutdown signal gracefully
	stopCh := signals.SetupSignalHandler()

	clientConfig, err := controller.BuildKubeConfig(kubeconfig)
	if err != nil {
		klog.Fatalf("Build kube config failed: %v", err)
	}

	isclientSet, err := isclientset.NewForConfig(clientConfig)
	if err != nil {
		klog.Fatalf("Error building imageset clientset: %v", err)
	}

	clientSet, err := clientset.NewForConfig(clientConfig)
	if err != nil {
		klog.Fatalf("Error building imageset clientset: %v", err)
	}

	InformerFactory := kubeinformers.NewSharedInformerFactory(clientSet, time.Second+30)
	isInformerFactory := informer.NewSharedInformerFactory(isclientSet, time.Second+30)



	hostName, err := imageset.GetHostName(hostnameOverride)
	if err != nil {
		klog.Fatalf("Get hostname failed: %v", err)
	}

	isc, err := imageset.NewImageSetController(
		InformerFactory.Apps().V1().Deployments(),
		InformerFactory.Apps().V1().StatefulSets(),
		isInformerFactory.Apps().V1alpha1().ImageSets(),
		clientSet,
		isclientSet,
		hostName,
	)
	if err != nil {
		klog.Fatalf("Error new ImageSetController: %v", err)
	}

	go isc.Run(5,stopCh)

	isInformerFactory.Start(stopCh)

	go app.StartHealthzServer(healthzHost, healthzPort)

	select {}

}

var (
	kubeconfig       string // Path to a kubeconfig. Only required if out-of-cluster
	hostnameOverride string // The name of the host
	healthzHost      string // The host of Healthz
	healthzPort      string // The port of Healthz to listen on
)

func init() {
	flag.StringVar(&kubeconfig, "kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
	flag.StringVar(&hostnameOverride, "hostnameOverride", "", "The name of the host")
	flag.StringVar(&healthzHost, "healthz-host", HealthzHost, "The host of Healthz.")
	flag.StringVar(&healthzPort, "healthz-port", HealthzPort, "The port of Healthz to listen on.")
}
