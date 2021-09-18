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

package imageset

import (
	"context"
	"fmt"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	appsinformers "k8s.io/client-go/informers/apps/v1"
	appslisters "k8s.io/client-go/listers/apps/v1"
	"time"

	putil "github.com/caoyingjunz/libpixiu/pixiu"
	dockertypes "github.com/docker/docker/api/types"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	v1core "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/component-base/metrics/prometheus/ratelimiter"
	"k8s.io/klog/v2"

	appsv1alpha1 "github.com/caoyingjunz/pixiu/pkg/apis/apps/v1alpha1"
	isClientset "github.com/caoyingjunz/pixiu/pkg/client/clientset/versioned"
	isInformers "github.com/caoyingjunz/pixiu/pkg/client/informers/externalversions/apps/v1alpha1"
	isListers "github.com/caoyingjunz/pixiu/pkg/client/listers/apps/v1alpha1"
	"github.com/caoyingjunz/pixiu/pkg/controller"
	"github.com/caoyingjunz/pixiu/pkg/controller/libdocker"

	"reflect"
)

const (
	maxRetries = 15

	AddEvent           string = "Add"
	UpdateEvent        string = "Update"
	DeleteEvent        string = "Delete"
	RecoverAddEvent    string = "RecoverAdd"
	RecoverUpdateEvent string = "RecoverUpdate"
	RecoverDeleteEvent string = "RecoverDelete"
	ImagesetEvent      string = "ImagesetEvent"

	markInnerEvent string = "markInnerEvent"

	Deployment  string = "Deployment"
	StatefulSet string = "StatefulSet"
)

var controllerKind = v1.SchemeGroupVersion.WithKind("ImageSet")

// ImageSetController is responsible for synchronizing images objects stored
// in the system.
type ImageSetController struct {
	client clientset.Interface
	isClient isClientset.Interface

	eventRecorder record.EventRecorder

	dc libdocker.Interface

	hostName string

	enqueueImageSet func(imageSet *appsv1alpha1.ImageSet)
	syncHandler func(iKey string) error

	dLister appslisters.DeploymentLister
	sLister	appslisters.StatefulSetLister
	isLister isListers.ImageSetLister

	dListerSynced cache.InformerSynced
	sListerSynced cache.InformerSynced
	isListerSynced cache.InformerSynced

	// ImageSet that need to be synced
	queue workqueue.RateLimitingInterface

	store SafeStoreInterface
}

func NewImageSetController(
	dInformer appsinformers.DeploymentInformer,
	sInformer appsinformers.StatefulSetInformer,
	isInformer isInformers.ImageSetInformer,
	client clientset.Interface,
	isClient isClientset.Interface,
	hostName string) (*ImageSetController, error) {
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(klog.Infof)
	eventBroadcaster.StartRecordingToSink(&v1core.EventSinkImpl{Interface: client.CoreV1().Events("")})

	if client != nil && client.CoreV1().RESTClient().GetRateLimiter() != nil {
		if err := ratelimiter.RegisterMetricAndTrackRateLimiterUsage("imageset_controller", client.CoreV1().RESTClient().GetRateLimiter()); err != nil {
			return nil, err
		}
	}

	isc := &ImageSetController{
		client:        client,
		isClient:      isClient,
		eventRecorder: eventBroadcaster.NewRecorder(scheme.Scheme, v1.EventSource{Component: "imageset-controller"}),
		hostName:      hostName,
		queue:         workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "imageset"),
		store:         newSafeStore(),
	}

	isc.dc = libdocker.ConnectToDockerOrDie(dockerHost, time.Duration(60)*time.Second, 1800)

	dInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    isc.addEvents,
		UpdateFunc: isc.updateEvents,
		DeleteFunc: isc.deleteEvents,
	})
	sInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    isc.addEvents,
		UpdateFunc: isc.updateEvents,
		DeleteFunc: isc.deleteEvents,
	})

	isInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    isc.addImageSet,
		UpdateFunc: isc.updateImageSet,
		DeleteFunc: isc.deleteImageSet,
	})

	isc.dLister = dInformer.Lister()
	isc.sLister = sInformer.Lister()
	isc.isLister = isInformer.Lister()

	isc.syncHandler = isc.syncImageSet
	isc.enqueueImageSet = isc.enqueue

	isc.dListerSynced = dInformer.Informer().HasSynced
	isc.sListerSynced = sInformer.Informer().HasSynced
	isc.isListerSynced = isInformer.Informer().HasSynced

	return isc, nil
}

func (isc *ImageSetController) Run(workers int, stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()
	defer isc.queue.ShutDown()

	klog.Infof("Starting imageSet Controller")
	defer klog.Infof("Shutting down imageSet Controller")

	// Wait for all involved caches to be synced, before processing items from the queue is started
	if !cache.WaitForNamedCacheSync("ImageSet-controller", stopCh, isc.isListerSynced) {
		return
	}

	for i := 0; i < workers; i++ {
		go wait.Until(isc.worker, time.Second, stopCh)
	}

	<-stopCh
}

// syncimageset will sync the imageset with the given key.
func (isc *ImageSetController) syncImageSet(key string) error {
	startTime := time.Now()
	klog.V(4).Infof("Started syncing imageSet %q (%v)", key, startTime)
	defer func() {
		klog.V(4).Infof("Finished syncing imageSet %q (%v)", key, time.Since(startTime))
	}()

	// Delete the obj from store even though the syncAutoscalers failed
	defer isc.store.Delete(key)

	is, exists := isc.store.Get(key)
	if !exists {
		// Do nothing and return directly
		return nil
	}

	var err error
	event := isc.popInnerEvent(is)
	klog.V(0).Infof("Handlering %s event for  %s",event, is)

	switch event {
	case AddEvent:
		_, err = isc.isClient.AppsV1alpha1().ImageSets(is.Namespace).Create(context.TODO(), is, metav1.CreateOptions{})
		if err != nil && !errors.IsAlreadyExists(err) {
			msg := fmt.Sprintf("Failed to create IS %s/%s for %s", is.Namespace, is.Name, is.Kind)

			isc.eventRecorder.Eventf(is, v1.EventTypeWarning, "FailedCreateImageSet", msg)
			return err
		}
		isc.eventRecorder.Eventf(is, v1.EventTypeNormal, "CreateImageSet",
			fmt.Sprintf("Create IS %s/%s for %s success", is.Namespace, is.Name, is.Kind))
	case UpdateEvent:
		_, err = isc.isClient.AppsV1alpha1().ImageSets(is.Namespace).Update(context.TODO(), is, metav1.UpdateOptions{})
		if err != nil && !errors.IsNotFound(err) {
			msg := fmt.Sprintf("Failed to update IS %s/%s for %s", is.Namespace, is.Name, is.Kind)
			isc.eventRecorder.Eventf(is, v1.EventTypeWarning, "FailedUpdateIS", msg)
			return err
		}
		isc.eventRecorder.Eventf(is, v1.EventTypeNormal, "UpdateIS",
			fmt.Sprintf("Update IS %s/%s for %s success", is.Namespace, is.Name, is.Kind))
	case DeleteEvent:
		err = isc.isClient.AppsV1alpha1().ImageSets(is.Namespace).Delete(context.TODO(), is.Name, metav1.DeleteOptions{})
		if err != nil && !errors.IsNotFound(err) {
			msg := fmt.Sprintf("Failed to delete IS %s/%s forr %s", is.Namespace, is.Name, is.Kind)
			isc.eventRecorder.Eventf(is, v1.EventTypeWarning, "FailedDeleteIS", msg)
			return err
		}
		isc.eventRecorder.Eventf(is, v1.EventTypeNormal, "DeleteIS",
			fmt.Sprintf("Delete IS %s/%s for %s success", is.Namespace, is.Name, is.Kind))
	case RecoverUpdateEvent:
		newIS, err := isc.GetNewestISFromResource(is)
		newIS.ResourceVersion = is.ResourceVersion
		if err != nil {
			msg := fmt.Sprintf("Failed to get update newest IS %s/%s for %s", is.Namespace, is.Name, is.Kind)
			isc.eventRecorder.Eventf(is, v1.EventTypeWarning, "FailedNewestIS", msg)
			return err
		}
		// no need to Recover IS from Update event
		if newIS == nil {
			return nil
		}
		if reflect.DeepEqual(is.Spec, newIS.Spec) {
			klog.V(2).Infof("IS: %s/%s spec is not changed, no need to updated", is.Namespace, is.Kind)
			return nil
		}
		_, err = isc.isClient.AppsV1alpha1().ImageSets(newIS.Namespace).Update(context.TODO(), newIS, metav1.UpdateOptions{})
		if err != nil && !errors.IsNotFound(err) {
			msg := fmt.Sprintf("Failed to Recover update IS %s/%s for %s", is.Namespace, is.Name, is.Kind)
			isc.eventRecorder.Eventf(is, v1.EventTypeWarning, "FailedRecoverIS", msg)
			return err
		}
		isc.eventRecorder.Eventf(is, v1.EventTypeNormal, "RecoverIS",
			fmt.Sprintf("Recover Update IS %s/%s for %s success", is.Namespace, is.Name, is.Kind))
	case RecoverDeleteEvent:
		newIS, err := isc.GetNewestISFromResource(is)
		if err != nil {
			msg := fmt.Sprintf("Failed to get delete newest IS %s/%s for %s", is.Namespace, is.Name, is.Kind)
			isc.eventRecorder.Eventf(is, v1.EventTypeWarning, "FailedNewestIS", msg)
			return err
		}
		// no need to Recover IS from Delete event
		if newIS == nil {
			return nil
		}
		_, err = isc.isClient.AppsV1alpha1().ImageSets(newIS.Namespace).Create(context.TODO(), newIS, metav1.CreateOptions{})
		if err != nil {
			msg := fmt.Sprintf("Failed to get delete newest IS %s/%s for %s", is.Namespace, is.Name, is.Kind)
			isc.eventRecorder.Eventf(is, v1.EventTypeWarning, "FailedCreateIS", msg)
			return err
		}
		isc.eventRecorder.Eventf(is, v1.EventTypeNormal, "RecoverIS",
			fmt.Sprintf("Recover Delete IS %s/%s for %s success", newIS.Namespace, newIS.Name, is.Kind))
	case ImagesetEvent:
		namespace, name, err := cache.SplitMetaNamespaceKey(key)
		if err != nil {
			return err
		}

		ims, err := isc.isLister.ImageSets(namespace).Get(name)
		if errors.IsNotFound(err) {
			// kubernetes will use finalizers to handler the delete event
			klog.V(4).Infof("Image Sets %v has been deleted", key)
			return nil
		}
		if err != nil {
			klog.Errorf("Get imageSet from index failed: %v", err)
			return err
		}
		if isc.isSelectedNode(ims) {
			klog.V(4).Infof("The non-local node does not pull the image")
			return nil
		}

		var imageRef string
		image := ims.Spec.Image

		// images action, only should pull and remove action, is equal to docker command
		switch ims.Spec.Action {
		case putil.Pull:
			if isc.dc.IsImageExists(image, dockertypes.ImageListOptions{}) &&
				putil.PullIfNotPresent == ims.Spec.ImagePullPolicy {
				klog.V(2).Infof("%s image %q already present on node %q.", putil.Pull, image, isc.hostName)
				return nil
			}

			auth := ims.Spec.Auth
			authConfig := dockertypes.AuthConfig{}
			if auth != nil {
				authConfig.Username = auth.Username
				authConfig.Password = auth.Password
				authConfig.ServerAddress = auth.ServerAddress
				authConfig.IdentityToken = auth.IdentityToken
				authConfig.RegistryToken = auth.RegistryToken
			}
			// TODO, add event supported
			// PullAlways or PullNotPresent
			klog.V(2).Infof("image %q will be pulling on node %q", image, isc.hostName)
			imageRef, err = isc.dc.PullImage(image, authConfig, dockertypes.ImagePullOptions{})
		case putil.Remove:
			if !isc.dc.IsImageExists(image, dockertypes.ImageListOptions{}) {
				klog.V(2).Infof("%s is unnecessary since image %q not exits on node %q.", putil.Remove, image, isc.hostName)
				return nil
			}

			klog.V(2).Infof("image %q will be removing from node %q", image, isc.hostName)
			_, err = isc.dc.RemoveImage(image, dockertypes.ImageRemoveOptions{})
		default:
			return fmt.Errorf("unsupported imageset action: %s", ims.Spec.Action)
		}

		if err != nil {
			klog.Errorf("%s imageset: %s failed: %v", ims.Spec.Action, image, err)
			return err
		}

		// TODO: need complete lock here
		newStatus := calculateImageSetStatus(isc.isClient.AppsV1alpha1().ImageSets(ims.Namespace), ims.Name, isc.hostName, imageRef, err)

		ims = ims.DeepCopy()
		// Always try to update as sync come up or failed.
		_, err = updateImageSetStatus(isc.isClient.AppsV1alpha1().ImageSets(ims.Namespace), ims, newStatus)
		if err != nil {
			klog.Errorf("update %s imageset: %s  status failed: %v", ims.Spec.Action, image, err)
			return err
		}

		klog.Infof("Imageset %q for image %q has been hanlder %q success", ims.Name, image, ims.Spec.Action)
		return nil
	default:
		return fmt.Errorf("Unsupported handlers event %s", event)
	}
	return err
}

// GetNewestIS will get newest IS from kubernetes resources
func (isc *ImageSetController) GetNewestISFromResource(
	is *appsv1alpha1.ImageSet) (*appsv1alpha1.ImageSet, error) {
	var annotations map[string]string
	var uid types.UID
	var image string
	kind := is.ObjectMeta.OwnerReferences[0].Kind
	switch kind {
	case Deployment:
		d, err := isc.dLister.Deployments(is.Namespace).Get(is.Name)
		if err != nil {
			return nil, err
		}
		// check
		if !IsOwnerReference(d.UID, is.OwnerReferences) {
			return nil, nil
		}
		uid = d.UID
		annotations = d.Annotations
		image = d.Spec.Template.Spec.Containers[0].Image
	case StatefulSet:
		s, err := isc.sLister.StatefulSets(is.Namespace).Get(is.Name)
		if err != nil {
			return nil, err
		}
		if !IsOwnerReference(s.UID, is.OwnerReferences) {
			return nil, nil
		}
		uid = s.UID
		annotations = s.Annotations
		image = s.Spec.Template.Spec.Containers[0].Image
	}
	if !IsNeedForIS(annotations) {
		return nil, nil
	}

	return CreateImageSet(
		is.Name, is.Namespace, APIVersion, kind, uid, annotations, image)
}

func (isc *ImageSetController) wrapInnerEvent(img *appsv1alpha1.ImageSet, event string) {
	// there is no necessary to lock the Annotations
	if img.Annotations == nil {
		img.Annotations = map[string]string{
			markInnerEvent: event,
		}
		return
	}
	img.Annotations[markInnerEvent] = event
}

// To pop kubez annotation and clean up kubez marker from HPA
func (isc *ImageSetController) popInnerEvent(img *appsv1alpha1.ImageSet) string {
	event, exists := img.Annotations[markInnerEvent]
	// This shouldn't happen, because we only insert annotation for hpa
	if exists {
		delete(img.Annotations, markInnerEvent)
	}
	return event
}

func (isc *ImageSetController) addEvents(obj interface{}) {
	isCtx := NewImageSetContext(obj)
	klog.V(2).Infof("Adding %s %s/%s", "ImageSet", isCtx.Namespace, isCtx.Name)

	if !IsNeedForIS(isCtx.Annotations) {
		return
	}

	is, err := CreateImageSet(
		isCtx.Name, isCtx.Namespace, isCtx.APIVersion, isCtx.Kind, isCtx.UID, isCtx.Annotations, isCtx.Image)
	if err != nil {
		utilruntime.HandleError(err)
		return
	}

	key, err := KeyFunc(is)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("couldn't get key for object %#v: %v", is, err))
		return
	}
	isc.wrapInnerEvent(is, AddEvent)
	isc.store.Update(key, is)

	isc.enqueueImageSet(is)
}

func (isc *ImageSetController) updateEvents(old, cur interface{}) {
	oldCtx := NewImageSetContext(old)
	curCtx := NewImageSetContext(cur)

	klog.V(2).Infof("Updating %s %s/%s", "ImageSet", oldCtx.Namespace, oldCtx.Namespace)

	if reflect.DeepEqual(oldCtx.Annotations, curCtx.Annotations) {
		return
	}

	oldExists := IsNeedForIS(oldCtx.Annotations)
	curExists := IsNeedForIS(curCtx.Annotations)

	//Delete IS
	if oldExists && !curExists {
		is, err := isc.isLister.ImageSets(oldCtx.Namespace).Get(oldCtx.Name)
		//judge the err state
		if err != nil {
			if errors.IsNotFound(err) {
				return
			}
			utilruntime.HandleError(err)
			return
		}

		key, err := KeyFunc(is)
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("couldn't get key for object %#v: %v", is, err))
			return
		}
		isc.wrapInnerEvent(is, DeleteEvent)
		isc.store.Add(key, is)

		isc.enqueueImageSet(is)
		return
	}

	// Add or Update IS
	is, err := CreateImageSet(curCtx.Name, curCtx.Namespace, curCtx.APIVersion, curCtx.Kind, curCtx.UID, curCtx.Annotations, curCtx.Image)
	if err != nil {
		utilruntime.HandleError(err)
		return
	}

	key, err := KeyFunc(is)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("couldn't get key for object %#v: %v", is, err))
		return
	}

	if !oldExists && curExists {
		isc.wrapInnerEvent(is, AddEvent)
	} else if oldExists && curExists {
		i, err := isc.isClient.AppsV1alpha1().ImageSets(is.Namespace).Get(context.TODO(), is.Name, metav1.GetOptions{})
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("couldn't get key for object %#v: %v", i, err))
			return
		}
		is.ObjectMeta.ResourceVersion = i.ResourceVersion
		isc.wrapInnerEvent(is, UpdateEvent)
	}
	isc.store.Add(key, is)

	isc.enqueueImageSet(is)
}

func (isc *ImageSetController) deleteEvents(obj interface{}) {
	isCtx := NewImageSetContext(obj)
	klog.V(2).Infof("Deleting %s %s/%s", "ImageSet", isCtx.Namespace, isCtx.Name)

	is, err := isc.isLister.ImageSets(isCtx.Namespace).Get(isCtx.Name)
	if err != nil {
		//judge the err state
		if errors.IsNotFound(err) {
			// IS has been deleted
			return
		}
		utilruntime.HandleError(err)
		return
	}

	if IsOwnerReference(isCtx.UID, is.OwnerReferences) {
		return
	}

	key, err := KeyFunc(is)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("couldn't get key for object %#v: %v", is, err))
		return
	}
	isc.wrapInnerEvent(is,DeleteEvent)
	isc.store.Update(key, is)

	isc.enqueueImageSet(is)
}

func (isc *ImageSetController) addImageSet(obj interface{}) {
	is := obj.(*appsv1alpha1.ImageSet)
		key, err := KeyFunc(is)
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("couldn't get key for object %#v: %v", is, err))
			return
		}
		isc.wrapInnerEvent(is, ImagesetEvent)
		isc.store.Update(key, is)

		klog.V(2).Infof("Adding ImageSet %s/%s", is.Namespace, is.Name)
		isc.enqueueImageSet(is)
}

func (isc *ImageSetController) updateImageSet(old, cur interface{}) {
	oldImageSet := old.(*appsv1alpha1.ImageSet)
	curImageSet := cur.(*appsv1alpha1.ImageSet)

	if oldImageSet.ResourceVersion == curImageSet.ResourceVersion {
		return
	}
	// Just update the status
	if oldImageSet.Generation == curImageSet.Generation {
		return
	}
	if !ManagerByKubezController(oldImageSet) {
		klog.V(2).Infof("Updating ImageSet %s/%s", curImageSet.Namespace, curImageSet.Name)

		key, err := KeyFunc(curImageSet)
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("couldn't get key for object %#v: %v", curImageSet, err))
			return
		}
		isc.wrapInnerEvent(curImageSet, ImagesetEvent)
		isc.store.Update(key, curImageSet)

		isc.enqueueImageSet(curImageSet)
	} else {
		klog.V(2).Infof("Updating ImageSet %s/%s", curImageSet.Namespace, curImageSet.Name)

		key, err := KeyFunc(curImageSet)
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("couldn't get key for object %#v: %v", curImageSet, err))
			return
		}
		isc.wrapInnerEvent(curImageSet, RecoverAddEvent)
		isc.store.Update(key, curImageSet)

		isc.enqueueImageSet(curImageSet)
	}
}

func (isc *ImageSetController) deleteImageSet(obj interface{}) {
	is := obj.(*appsv1alpha1.ImageSet)
	if !ManagerByKubezController(is) {
		key, err := KeyFunc(is)
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("couldn't get key for object %#v: %v", is, err))
			return
		}
		isc.wrapInnerEvent(is, ImagesetEvent)
		isc.store.Update(key, is)

		isc.enqueueImageSet(is)
	} else {
		klog.V(0).Infof("Deleting IS %s/%s", is.Namespace, is.Name)

		key, err := KeyFunc(is)
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("couldn't get key for object %#v: %v", is, err))
			return
		}
		isc.wrapInnerEvent(is, RecoverDeleteEvent)
		isc.store.Update(key, is)

		isc.enqueueImageSet(is)
	}

	klog.V(2).Infof("deleting ImageSet %s/%s", is.Namespace, is.Name)
}

func (isc *ImageSetController) isSelectedNode(ims *appsv1alpha1.ImageSet) bool {
	var isSelected bool
	nodes := ims.Spec.Selector.Nodes
	for _, node := range nodes {
		if isc.hostName == node {
			isSelected = true
			break
		}
	}
	return isSelected
}

func (isc *ImageSetController) enqueue(imageSet *appsv1alpha1.ImageSet) {
	key, err := controller.KeyFunc(imageSet)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("Couldn't get key for object %#v: %v", imageSet, err))
		return
	}

	isc.queue.Add(key)
}

// worker runs a worker thread that just dequeues items, processes then, and marks them done.
func (isc *ImageSetController) worker() {
	for isc.processNextWorkItem() {
	}
}

func (isc *ImageSetController) processNextWorkItem() bool {
	key, quit := isc.queue.Get()
	if quit {
		return false
	}
	defer isc.queue.Done(key)

	err := isc.syncHandler(key.(string))
	isc.handleErr(err, key)

	return true
}

func (isc *ImageSetController) handleErr(err error, key interface{}) {
	if err == nil {
		isc.queue.Forget(key)
		return
	}

	if isc.queue.NumRequeues(key) < maxRetries {
		klog.V(2).Infof("Error syncing image for imageset %q, retrying. Error: %v", key, err)
		isc.queue.AddRateLimited(key)
		return
	}

	klog.Warningf("Dropping imageset %q out of the queue: %v", key, err)
	utilruntime.HandleError(err)
	isc.queue.Forget(key)
}