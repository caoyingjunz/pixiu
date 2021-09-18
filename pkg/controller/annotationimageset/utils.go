package annotationimageset

import (
	"github.com/caoyingjunz/libpixiu/pixiu"
	appsv1alpha1 "github.com/caoyingjunz/pixiu/pkg/apis/apps/v1alpha1"
	apps "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

var (
	KeyFunc = cache.DeletionHandlingMetaNamespaceKeyFunc
)
const (
	host				    string = "kube-node"
	imageset 			    string = "ImageSet"
	imagesetAPIVersion      string = "apps.pixiu.io/v1alpha1"
	APIVersion              string = "apps/v1"
)

type AnnotationImageSetContext struct {
	Name        string            `json:"name"`
	Namespace   string            `json:"namespace"`
	APIVersion  string            `json:"api_version"`
	Kind        string            `json:"kind"`
	UID        types.UID          `json:"uid"`
	Annotations map[string]string `json:"annotations"`
	Image       string	  		  `json:"Image"`
}

func NewAnnotationImageSetContext(obj interface{}) *AnnotationImageSetContext {
	// TODO: 后续优化，直接获取 hpa 的 Annotations

	switch o := obj.(type) {
	case *apps.Deployment:
		return &AnnotationImageSetContext{
			Name:        o.Name,
			Namespace:   o.Namespace,
			APIVersion:  APIVersion,
			Kind:        Deployment,
			UID:         o.UID,
			Annotations: o.Annotations,
			Image:  o.Spec.Template.Spec.Containers[0].Image,
		}
	case *apps.StatefulSet:
		return &AnnotationImageSetContext{
			Name:        o.Name,
			Namespace:   o.Namespace,
			APIVersion:  APIVersion,
			Kind:        StatefulSet,
			UID:         o.UID,
			Annotations: o.Annotations,
			Image:  o.Spec.Template.Spec.Containers[0].Image,
		}
	default:
		// never happens
		return nil
	}
}

func CreateImageSet(
	name string,
	namespace string,
	apiVersion string,
	kind string,
	uid types.UID,
	annotations map[string]string,
	Image string) (*appsv1alpha1.ImageSet, error) {

	err := CheckAnnotation(annotations)
	if err != nil  {
		klog.Errorf("Extract from annotations failed")
		return nil, err
	}

	controller := true
	blockOwnerDeletion := true
	ownerReference := metav1.OwnerReference{
		APIVersion:         apiVersion,
		Kind:               kind,
		Name:               name,
		UID:                uid,
		Controller:         &controller,
		BlockOwnerDeletion: &blockOwnerDeletion,
	}

	img := &appsv1alpha1.ImageSet{
		TypeMeta: metav1.TypeMeta{
			Kind:       imageset,
			APIVersion: imagesetAPIVersion,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			OwnerReferences: []metav1.OwnerReference{
				ownerReference,
			},
		},
		Spec: appsv1alpha1.ImageSetSpec{
			Image: Image,
			Action: annotations[Annotation],
			ImagePullPolicy: pixiu.PullIfNotPresent,
			Selector: appsv1alpha1.NodeSelector{
				Nodes: []string{host},
			},
			Auth: &appsv1alpha1.AuthConfig{
			},
		},
	}
	return img, nil
}

func IsOwnerReference(uid types.UID, ownerReferences []metav1.OwnerReference) bool {
	var isOwnerRef bool
	for _, ownerReferences := range ownerReferences {
		if uid == ownerReferences.UID {
			isOwnerRef = true
			break
		}
	}
	return isOwnerRef
}

func ManagerByKubezController(is *appsv1alpha1.ImageSet) bool {
	for _, managedField := range is.ManagedFields {
		if managedField.APIVersion == imagesetAPIVersion &&
			(managedField.Manager == KubezManager ||
				// This condition used for local run
				managedField.Manager == KubezMain) {
			return true
		}
	}
	return false
}
