package annotationimageset

import (
	"context"
	"fmt"
	appsv1alpha1 "github.com/caoyingjunz/pixiu/pkg/apis/apps/v1alpha1"
	isClientset "github.com/caoyingjunz/pixiu/pkg/client/clientset/versioned"
	pInformers "github.com/caoyingjunz/pixiu/pkg/client/informers/externalversions/apps/v1alpha1"
	isListers "github.com/caoyingjunz/pixiu/pkg/client/listers/apps/v1alpha1"
	"github.com/caoyingjunz/pixiu/pkg/controller"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	appsinformers "k8s.io/client-go/informers/apps/v1"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	v1core "k8s.io/client-go/kubernetes/typed/core/v1"
	appslisters "k8s.io/client-go/listers/apps/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/component-base/metrics/prometheus/ratelimiter"
	"k8s.io/klog/v2"
	"reflect"
	"time"
)

const (
	maxRetries = 15

	AddEvent           string = "Add"
	UpdateEvent        string = "Update"
	DeleteEvent        string = "Delete"
	RecoverDeleteEvent string = "RecoverDelete"
	RecoverUpdateEvent string = "RecoverUpdate"

	markInnerEvent string = "markInnerEvent"

	Deployment  string = "Deployment"
	StatefulSet string = "StatefulSet"
)

type AnnotationImageSetController struct {
	client        clientset.Interface
	isClient 	  isClientset.Interface

	eventRecorder record.EventRecorder

	enqueueAnnotationImageSet func(img *appsv1alpha1.ImageSet)
	syncHandler     func(iKey string) error

	dLister appslisters.DeploymentLister
	sLister appslisters.StatefulSetLister
	isLister	isListers.ImageSetLister

	dListerSynced cache.InformerSynced
	sListerSynced cache.InformerSynced
	isListerSynced cache.InformerSynced

	// ImageSet that need to be synced
	queue workqueue.RateLimitingInterface

	store SafeStoreInterface
}

// NewAnnotationimagesetController creates a new AnnotationimagesetController.
func NewAnnotationimagesetController(
	dInformer appsinformers.DeploymentInformer,
	sInformer appsinformers.StatefulSetInformer,
	isInformer pInformers.ImageSetInformer,
	client clientset.Interface,
	isClient isClientset.Interface) (*AnnotationImageSetController, error) {
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(klog.Infof)
	eventBroadcaster.StartRecordingToSink(&v1core.EventSinkImpl{Interface: client.CoreV1().Events("")})

	if client != nil && client.CoreV1().RESTClient().GetRateLimiter() != nil {
		if err := ratelimiter.RegisterMetricAndTrackRateLimiterUsage("Annotationimageset_Controller", client.CoreV1().RESTClient().GetRateLimiter()); err != nil {
			return nil, err
		}
	}

	ais := &AnnotationImageSetController{
		client: client,
		isClient: isClient,
		eventRecorder: eventBroadcaster.NewRecorder(scheme.Scheme, v1.EventSource{Component: "Annotationimageset-Controller"}),
		queue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(),"Annotationimageset"),
		store: newSafeStore(),
	}

	dInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    ais.addEvents,
		UpdateFunc: ais.updateEvents,
		DeleteFunc: ais.deleteEvents,
	})
	sInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    ais.addEvents,
		UpdateFunc: ais.updateEvents,
		DeleteFunc: ais.deleteEvents,
	})
	isInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    ais.addIS,
		UpdateFunc: ais.updateIS,
		DeleteFunc: ais.deleteIS,
	})

	ais.dLister = dInformer.Lister()
	ais.sLister = sInformer.Lister()
	ais.isLister = isInformer.Lister()

	//syncAnnotationimageset
	ais.syncHandler = ais.syncAnnotationImageSet
	ais.enqueueAnnotationImageSet = ais.enqueue


	ais.dListerSynced = dInformer.Informer().HasSynced
	ais.sListerSynced = sInformer.Informer().HasSynced
	ais.isListerSynced = isInformer.Informer().HasSynced

	return ais, nil
}

// Run begins watching and syncing.
func (ais *AnnotationImageSetController) Run(workers int, stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()
	defer ais.queue.ShutDown()

	klog.Infof("Starting AnnotationImageSet Controller")
	defer klog.Infof("Shutting Down AnnotationImageSet Controller")

	// Wait for all involved caches to be synced, before processing items from the queue is started
	if !cache.WaitForNamedCacheSync("AnnotationImageSet-Manager", stopCh, ais.dListerSynced, ais.sListerSynced) {
		return
	}

	for i := 0; i < workers; i++ {
		go wait.Until(ais.worker, time.Second, stopCh)
	}

	<-stopCh
}

// syncAnnotationimageset will sync the imageset with the given key.
func (ais *AnnotationImageSetController) syncAnnotationImageSet(key string) error {
	starTime := time.Now()
	klog.V(2).Infof("Start syncing Imageset %q (%v)", key, starTime)
	defer func() {
		klog.V(2).Infof("Finished syncing Imageset %q (%v)", key, time.Since(starTime))
	}()

	// Delete the obj from store even though the syncAutoscalers failed
	defer ais.store.Delete(key)

	is, exists := ais.store.Get(key)
	if !exists {
		// Do nothing and return directly
		return nil
	}

	//kind := img.Kind

	var err error
	event := ais.popInnerEvent(is)
	//klog.V(0).Infof("Handlering %s event for %s/%s from %s", event, img.Namespace, img.Name, kind)
	klog.V(0).Infof("Handlering %s event for  %s",event, is)

	switch event {
	case AddEvent:
		_, err = ais.isClient.AppsV1alpha1().ImageSets(is.Namespace).Create(context.TODO(),is,metav1.CreateOptions{})
		if err != nil && !errors.IsAlreadyExists(err) {
			msg := fmt.Sprintf("Failed to create IS %s/%s for %s", is.Namespace, is.Name, is.Kind)

			ais.eventRecorder.Eventf(is, v1.EventTypeWarning, "FailedCreateImageSet", msg)
			return err
		}
		ais.eventRecorder.Eventf(is, v1.EventTypeNormal, "CreateImageSet",
			fmt.Sprintf("Create IS %s/%s for %s success", is.Namespace, is.Name, is.Kind))
	case UpdateEvent:
		_, err = ais.isClient.AppsV1alpha1().ImageSets(is.Namespace).Update(context.TODO(), is, metav1.UpdateOptions{})
		if err != nil && !errors.IsNotFound(err) {
			msg := fmt.Sprintf("Failed to update IS %s/%s for %s", is.Namespace, is.Name, is.Kind)
			ais.eventRecorder.Eventf(is, v1.EventTypeWarning, "FailedUpdateIS", msg)
			return err
		}
		ais.eventRecorder.Eventf(is, v1.EventTypeNormal, "UpdateIS",
			fmt.Sprintf("Update IS %s/%s for %s success", is.Namespace, is.Name, is.Kind))
	case DeleteEvent:
		err = ais.isClient.AppsV1alpha1().ImageSets(is.Namespace).Delete(context.TODO(), is.Name, metav1.DeleteOptions{})
		if err != nil && !errors.IsNotFound(err) {
			msg := fmt.Sprintf("Failed to delete IS %s/%s forr %s", is.Namespace, is.Name, is.Kind)
			ais.eventRecorder.Eventf(is, v1.EventTypeWarning, "FailedDeleteIS", msg)
			return err
		}
		ais.eventRecorder.Eventf(is, v1.EventTypeNormal, "DeleteIS",
			fmt.Sprintf("Delete IS %s/%s for %s success", is.Namespace, is.Name, is.Kind))
	case RecoverUpdateEvent:
		newIS, err := ais.GetNewestISFromResource(is)
		newIS.ResourceVersion = is.ResourceVersion
		if err != nil {
			msg := fmt.Sprintf("Failed to get update newest IS %s/%s for %s", is.Namespace, is.Name, is.Kind)
			ais.eventRecorder.Eventf(is, v1.EventTypeWarning, "FailedNewestIS", msg)
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
		_, err = ais.isClient.AppsV1alpha1().ImageSets(newIS.Namespace).Update(context.TODO(), newIS, metav1.UpdateOptions{})
		if err != nil && !errors.IsNotFound(err) {
			msg := fmt.Sprintf("Failed to Recover update IS %s/%s for %s", is.Namespace, is.Name, is.Kind)
			ais.eventRecorder.Eventf(is, v1.EventTypeWarning, "FailedRecoverIS", msg)
			return err
		}
		ais.eventRecorder.Eventf(is, v1.EventTypeNormal, "RecoverIS",
			fmt.Sprintf("Recover Update IS %s/%s for %s success", is.Namespace, is.Name, is.Kind))
	case RecoverDeleteEvent:
		newIS, err := ais.GetNewestISFromResource(is)
		if err != nil {
			msg := fmt.Sprintf("Failed to get delete newest IS %s/%s for %s", is.Namespace, is.Name, is.Kind)
			ais.eventRecorder.Eventf(is, v1.EventTypeWarning, "FailedNewestIS", msg)
			return err
		}
		// no need to Recover IS from Delete event
		if newIS == nil {
			return nil
		}
		_, err = ais.isClient.AppsV1alpha1().ImageSets(newIS.Namespace).Create(context.TODO(), newIS, metav1.CreateOptions{})
		if err != nil {
			msg := fmt.Sprintf("Failed to get delete newest IS %s/%s for %s", is.Namespace, is.Name, is.Kind)
			ais.eventRecorder.Eventf(is, v1.EventTypeWarning, "FailedCreateIS", msg)
			return err
		}
		ais.eventRecorder.Eventf(is, v1.EventTypeNormal, "RecoverIS",
			fmt.Sprintf("Recover Delete IS %s/%s for %s success", newIS.Namespace, newIS.Name, is.Kind))
	default:
		return fmt.Errorf("Unsupported handlers event %s", event)
	}
	return err
}

// GetNewestIS will get newest IS from kubernetes resources
func (ais *AnnotationImageSetController) GetNewestISFromResource(
	is *appsv1alpha1.ImageSet) (*appsv1alpha1.ImageSet, error) {
	var annotations map[string]string
	var uid types.UID
	var image string
	kind := is.ObjectMeta.OwnerReferences[0].Kind
	switch kind {
	case Deployment:
		d, err := ais.dLister.Deployments(is.Namespace).Get(is.Name)
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
		s, err := ais.sLister.StatefulSets(is.Namespace).Get(is.Name)
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

func (ais *AnnotationImageSetController) wrapInnerEvent(img *appsv1alpha1.ImageSet, event string) {
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
func (ais *AnnotationImageSetController) popInnerEvent(img *appsv1alpha1.ImageSet) string {
	event, exists := img.Annotations[markInnerEvent]
	// This shouldn't happen, because we only insert annotation for hpa
	if exists {
		delete(img.Annotations, markInnerEvent)
	}
	return event
}

func (ais *AnnotationImageSetController) addEvents(obj interface{}) {
	isCtx := NewAnnotationImageSetContext(obj)
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
	ais.wrapInnerEvent(is, AddEvent)
	ais.store.Update(key, is)

	ais.enqueueAnnotationImageSet(is)
}

func (ais *AnnotationImageSetController) updateEvents(old, cur interface{}) {
	oldCtx := NewAnnotationImageSetContext(old)
	curCtx := NewAnnotationImageSetContext(cur)

	klog.V(2).Infof("Updating %s %s/%s", "ImageSet", oldCtx.Namespace, oldCtx.Namespace)

	if reflect.DeepEqual(oldCtx.Annotations, curCtx.Annotations) {
		return
	}

	oldExists := IsNeedForIS(oldCtx.Annotations)
	curExists := IsNeedForIS(curCtx.Annotations)

	//Delete IS
	if oldExists && !curExists {
		is, err := ais.isLister.ImageSets(oldCtx.Namespace).Get(oldCtx.Name)
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
		ais.wrapInnerEvent(is, DeleteEvent)
		ais.store.Add(key, is)

		ais.enqueueAnnotationImageSet(is)
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
		ais.wrapInnerEvent(is, AddEvent)
	} else if oldExists && curExists {
		i, err := ais.isClient.AppsV1alpha1().ImageSets(is.Namespace).Get(context.TODO(), is.Name, metav1.GetOptions{})
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("couldn't get key for object %#v: %v", i, err))
			return
		}
		is.ObjectMeta.ResourceVersion = i.ResourceVersion
		ais.wrapInnerEvent(is, UpdateEvent)
	}
	ais.store.Add(key, is)

	ais.enqueueAnnotationImageSet(is)
}

func (ais *AnnotationImageSetController) deleteEvents(obj interface{}) {
	isCtx := NewAnnotationImageSetContext(obj)
	klog.V(2).Infof("Deleting %s %s/%s", "ImageSet", isCtx.Namespace, isCtx.Name)

	is, err := ais.isLister.ImageSets(isCtx.Namespace).Get(isCtx.Name)
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
	ais.wrapInnerEvent(is,DeleteEvent)
	ais.store.Update(key, is)

	ais.enqueueAnnotationImageSet(is)
}

func (ais *AnnotationImageSetController) addIS(obj interface{}) {
	i := obj.(*appsv1alpha1.ImageSet)
	if !ManagerByKubezController(i) {
		return
	}
	klog.V(0).Infof("Adding IS(manager by kubez) %s/%s", i.Namespace,i.Name)
}

func (ais *AnnotationImageSetController) updateIS(old, cur interface{}) {
	oldI := old.(*appsv1alpha1.ImageSet)
	curI := cur.(*appsv1alpha1.ImageSet)
	if oldI == curI {
		return
	}
	if !ManagerByKubezController(oldI) {
		return
	}
	klog.V(0).Infof("Updating IS %s/%s", oldI.Namespace, oldI.Name)

	key, err := KeyFunc(curI)
	if err != nil {
		return
	}
	ais.wrapInnerEvent(curI, RecoverUpdateEvent)
	ais.store.Update(key, curI)

	ais.enqueueAnnotationImageSet(curI)
}

func (ais *AnnotationImageSetController) deleteIS(obj interface{}) {
	i := obj.(*appsv1alpha1.ImageSet)
	if !ManagerByKubezController(i) {
		return
	}
	klog.V(0).Infof("Deleting IS %s/%s", i.Namespace, i.Name)

	key, err := KeyFunc(i)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("couldn't get key for object %#v: %v", i, err))
		return
	}
	ais.wrapInnerEvent(i, RecoverDeleteEvent)
	ais.store.Update(key, i)

	ais.enqueueAnnotationImageSet(i)
}


func (ais *AnnotationImageSetController) enqueue(is *appsv1alpha1.ImageSet) {
	key, err := controller.KeyFunc(is)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("couldn't get key for object %#v: %v", is, err))
		return
	}
	ais.queue.Add(key)
}

func (ais *AnnotationImageSetController) worker() {
	for ais.processNextWorkItem() {
	}
}

func (ais *AnnotationImageSetController) processNextWorkItem() bool {
	key, quit := ais.queue.Get()
	if quit {
		return false
	}
	defer ais.queue.Done(key)

	err := ais.syncHandler(key.(string))
	ais.handleErr(err, key)
	return true
}

func (ais *AnnotationImageSetController) handleErr(err error, key interface{}) {
	if err == nil {
		ais.queue.Forget(key)
		return
	}

	if ais.queue.NumRequeues(key) < maxRetries {
		klog.V(0).Infof("Error syncing IS %v: %v", key, err)
		ais.queue.AddRateLimited(key)
		return
	}

	utilruntime.HandleError(err)
	klog.V(0).Infof("Dropping IS %q out of the queue: %v", key, err)
	ais.queue.Forget(key)
}