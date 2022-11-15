package kubernetes

import (
	"context"

	v1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/caoyingjunz/gopixiu/api/meta"
	pixiuerrors "github.com/caoyingjunz/gopixiu/pkg/errors"
	"github.com/caoyingjunz/gopixiu/pkg/log"
)

type DaemonSetGetter interface {
	DaemonSets(cloud string) DaemonSetInterface
}

type DaemonSetInterface interface {
	Create(ctx context.Context, daemonset *v1.DaemonSet) error
	Update(ctx context.Context, daemonset *v1.DaemonSet) error
	Delete(ctx context.Context, deleteOptions meta.DeleteOptions) error
	Get(ctx context.Context, getOptions meta.GetOptions) (*v1.DaemonSet, error)
	List(ctx context.Context, listOptions meta.ListOptions) ([]v1.DaemonSet, error)
}

type daemonSets struct {
	client *kubernetes.Clientset
	cloud  string
}

func NewDaemonSets(client *kubernetes.Clientset, cloud string) DaemonSetInterface {
	return &daemonSets{
		client: client,
		cloud:  cloud,
	}
}

func (c *daemonSets) Create(ctx context.Context, daemonset *v1.DaemonSet) error {
	if c.client == nil {
		return pixiuerrors.ErrCloudNotRegister
	}
	if _, err := c.client.AppsV1().
		DaemonSets(daemonset.Namespace).
		Create(ctx, daemonset, metav1.CreateOptions{}); err != nil {
		log.Logger.Errorf("failed to delete %s namespace %s: %v", c.cloud, daemonset.Namespace, err)

		return err
	}

	return nil
}

func (c *daemonSets) Update(ctx context.Context, daemonset *v1.DaemonSet) error {
	if c.client == nil {
		return pixiuerrors.ErrCloudNotRegister
	}
	if _, err := c.client.AppsV1().
		DaemonSets(daemonset.Namespace).
		Update(ctx, daemonset, metav1.UpdateOptions{}); err != nil {
		log.Logger.Errorf("failed to update %s daemonset: %v", c.cloud, err)
		return err
	}

	return nil
}

func (c *daemonSets) Delete(ctx context.Context, deleteOptions meta.DeleteOptions) error {
	if c.client == nil {
		return pixiuerrors.ErrCloudNotRegister
	}
	if err := c.client.AppsV1().
		DaemonSets(deleteOptions.Namespace).
		Delete(ctx, deleteOptions.ObjectName, metav1.DeleteOptions{}); err != nil {
		log.Logger.Errorf("failed to delete %s daemonset: %v", deleteOptions.Namespace, err)
		return err
	}

	return nil
}

func (c *daemonSets) Get(ctx context.Context, getOptions meta.GetOptions) (*v1.DaemonSet, error) {
	if c.client == nil {
		return nil, pixiuerrors.ErrCloudNotRegister
	}
	ds, err := c.client.AppsV1().
		DaemonSets(getOptions.Namespace).
		Get(ctx, getOptions.ObjectName, metav1.GetOptions{})
	if err != nil {
		log.Logger.Errorf("failed to get %s daemonsets: %v", getOptions.Cloud, err)
		return nil, err
	}

	return ds, err
}

func (c *daemonSets) List(ctx context.Context, listOptions meta.ListOptions) ([]v1.DaemonSet, error) {
	if c.client == nil {
		return nil, pixiuerrors.ErrCloudNotRegister
	}
	ds, err := c.client.AppsV1().
		DaemonSets(listOptions.Namespace).
		List(ctx, metav1.ListOptions{})
	if err != nil {
		log.Logger.Errorf("failed to list %s daemonsets: %v", listOptions.Namespace, err)
		return nil, err
	}

	return ds.Items, nil
}
