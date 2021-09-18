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

// Package app implements a Server object for running the pixiu.
package app

import (
	"fmt"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/runtime/schema"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/wait"
	cacheddiscovery "k8s.io/client-go/discovery/cached"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/metadata"
	"k8s.io/client-go/metadata/metadatainformer"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"k8s.io/klog/v2"

	pClientset "github.com/caoyingjunz/pixiu/pkg/client/clientset/versioned"
	pInformers "github.com/caoyingjunz/pixiu/pkg/client/informers/externalversions"
	"github.com/caoyingjunz/pixiu/pkg/controller"
	"github.com/caoyingjunz/pixiu/pkg/controller/advanceddeployment"
	"github.com/caoyingjunz/pixiu/pkg/controller/advancedimage"
	"github.com/caoyingjunz/pixiu/pkg/controller/autoscaler"
)

const (
	workers = 5

	pixiuVersion     = "v1alpha1"
	pixiuGroup       = "apps.pixiu.io"
	pixiuAllFeatures = "advancedDeployment,autoscaler,advancedImage"
)

// ControllerContext defines the context obj for pixiu
type ControllerContext struct {
	// ClientBuilder will provide a client for this controller to use
	ClientBuilder controller.ControllerClientBuilder
	// PixiuClientBuilder will provide a client for pixiu controller to use
	PixiuClientBuilder func(c *rest.Config) *pClientset.Clientset

	// InformerFactory gives access to informers for the controller.
	InformerFactory informers.SharedInformerFactory

	ObjectOrMetadataInformerFactory controller.InformerFactory

	PixiuInformerFactory pInformers.SharedInformerFactory

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

func CreateControllerContext(clientBuilder controller.ControllerClientBuilder, kubeConfig *rest.Config, featureGates string, stop <-chan struct{}) (ControllerContext, error) {
	versionedClient := clientBuilder.ClientOrDie("shared-informers")
	sharedInformers := informers.NewSharedInformerFactory(versionedClient, time.Minute)

	metadataClient := metadata.NewForConfigOrDie(clientBuilder.ConfigOrDie("metadata-informers"))
	metadataInformers := metadatainformer.NewSharedInformerFactory(metadataClient, time.Minute)

	pixiuClient := pClientset.NewForConfigOrDie(kubeConfig)
	pixiuInformers := pInformers.NewSharedInformerFactory(pixiuClient, time.Second+30)

	availableResources, err := GetAvailableResources(featureGates)
	if err != nil {
		return ControllerContext{}, err
	}

	// If APIServer is not runnint we should wait for some time unless failed
	if err := WaitForAPIServer(versionedClient, time.Second*8); err != nil {
		return ControllerContext{}, err
	}

	discoveryClient := clientBuilder.ClientOrDie("controller-discovery")
	cachedClient := cacheddiscovery.NewMemCacheClient(discoveryClient.Discovery())
	restMapper := restmapper.NewDeferredDiscoveryRESTMapper(cachedClient)
	go wait.Until(func() {
		restMapper.Reset()
	}, 30*time.Second, stop)

	ctx := ControllerContext{
		ClientBuilder:                   clientBuilder,
		PixiuClientBuilder:              pClientset.NewForConfigOrDie,
		InformerFactory:                 sharedInformers,
		ObjectOrMetadataInformerFactory: controller.NewInformerFactory(sharedInformers, metadataInformers),
		PixiuInformerFactory:            pixiuInformers,
		AvailableResources:              availableResources,
		KubeConfig:                      kubeConfig,
		Stop:                            stop,
	}
	return ctx, nil
}

// StartControllers starts a set of controllers with a specified ControllerContext
func StartControllers(ctx ControllerContext, controllers map[string]InitFunc) error {
	for controllerName, intFn := range controllers {
		klog.V(0).Infof("Starting %q", controllerName)

		started, err := intFn(ctx)
		if err != nil {
			klog.Errorf("Starting %q failed", controllerName)
			return err
		}
		if !started {
			klog.Warningf("Skipping %q", controllerName)
			continue
		}
	}

	return nil
}

var allControllers = map[string]bool{
	"advancedDeployment": true,
	"advancedImage":      true,
	"autoscaler":         true,
}

// GetAvailableResources gets the map which contains all Piuxiu available resources
func GetAvailableResources(featureGates string) (map[schema.GroupVersionResource]bool, error) {
	if len(strings.TrimSpace(featureGates)) == 0 {
		featureGates = pixiuAllFeatures
	}

	var errs []error
	allResources := map[schema.GroupVersionResource]bool{}
	for _, feature := range strings.Split(featureGates, ",") {
		feature = strings.TrimSpace(feature)
		if !allControllers[feature] {
			errs = append(errs, fmt.Errorf("unsupported feature %q", feature))
			continue
		}
		allResources[schema.GroupVersionResource{
			Group:    pixiuGroup,
			Version:  pixiuVersion,
			Resource: feature,
		}] = true
	}

	return allResources, utilerrors.NewAggregate(errs)
}

type InitFunc func(ctx ControllerContext) (enabled bool, err error)

// NewControllerInitializers is a public map of named controller groups
func NewControllerInitializers() map[string]InitFunc {
	controllers := map[string]InitFunc{}
	controllers["advancedDeployment"] = startPixiuController
	controllers["advancedImage"] = startAdvancedImageController
	controllers["autoscaler"] = startAutoscalerController

	return controllers
}

func startPixiuController(ctx ControllerContext) (bool, error) {
	if !ctx.AvailableResources[schema.GroupVersionResource{Group: pixiuGroup, Version: pixiuVersion, Resource: "advancedDeployment"}] {
		return false, nil
	}
	pc, err := advanceddeployment.NewPixiuController(
		ctx.PixiuClientBuilder(ctx.KubeConfig),
		ctx.PixiuInformerFactory.Apps().V1alpha1().AdvancedDeployments(),
		ctx.InformerFactory.Core().V1().Pods(),
		ctx.ClientBuilder.ClientOrDie("shared-informers"),
	)
	if err != nil {
		return true, fmt.Errorf("New pixiu controller failed: %v", err)
	}

	go pc.Run(workers, ctx.Stop)
	return true, nil
}

func startAutoscalerController(ctx ControllerContext) (bool, error) {
	if !ctx.AvailableResources[schema.GroupVersionResource{Group: pixiuGroup, Version: pixiuVersion, Resource: "autoscaler"}] {
		return false, nil
	}
	ac, err := autoscaler.NewAutoscalerController(
		ctx.InformerFactory.Apps().V1().Deployments(),
		ctx.InformerFactory.Apps().V1().StatefulSets(),
		ctx.InformerFactory.Autoscaling().V2beta2().HorizontalPodAutoscalers(),
		ctx.ClientBuilder.ClientOrDie("shared-informer"),
	)
	if err != nil {
		return true, fmt.Errorf("New autoscaler controller failed: %v", err)
	}

	go ac.Run(workers, ctx.Stop)
	return true, nil
}

func startAdvancedImageController(ctx ControllerContext) (bool, error) {
	if !ctx.AvailableResources[schema.GroupVersionResource{Group: pixiuGroup, Version: pixiuVersion, Resource: "advancedImage"}] {
		return false, nil
	}
	ai, err := advancedimage.NewAdvancedImageController(
		ctx.PixiuClientBuilder(ctx.KubeConfig),
		ctx.PixiuInformerFactory.Apps().V1alpha1().AdvancedImages(),
		ctx.PixiuInformerFactory.Apps().V1alpha1().ImageSets(),
		ctx.ClientBuilder.ClientOrDie("shared-informer"),
	)
	if err != nil {
		return true, fmt.Errorf("New advancedImage controller failed: %v", err)
	}

	go ai.Run(workers, ctx.Stop)
	return true, nil
}