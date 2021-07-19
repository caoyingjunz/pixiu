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

package libdocker

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	containerdapi "github.com/containerd/containerd"
	cns "github.com/containerd/containerd/namespaces"
	dockertypes "github.com/docker/docker/api/types"
	dockerapi "github.com/docker/docker/client"
	"k8s.io/klog/v2"
)

const (
	// defaultTimeout is the default timeout of short running docker operations.
	// Value is slightly offset from 2 minutes to make timeouts due to this
	// constant recognizable.
	defaultTimeout = 2*time.Minute - 1*time.Second

	// defaultShmSize is the default ShmSize to use (in bytes) if not specified.
	defaultShmSize = int64(1024 * 1024 * 64)

	// defaultImagePullingProgressReportInterval is the default interval of image pulling progress reporting.
	defaultImagePullingProgressReportInterval = 10 * time.Second
)

// Interface is an abstract interface for testability. It abstracts the interface of docker client.
type Interface interface {
	PullImage(image string, auth dockertypes.AuthConfig, opts dockertypes.ImagePullOptions) (string, error)
	RemoveImage(image string, opts dockertypes.ImageRemoveOptions) ([]dockertypes.ImageDeleteResponseItem, error)
	InspectImageByRef(imageRef string) (*dockertypes.ImageInspect, error)
	IsImageExists(image string, opts dockertypes.ImageListOptions) bool
	//ListImages(opts dockertypes.ImageListOptions) ([]dockertypes.ImageSummary, error)
	//InspectImageByID(imageID string) (*dockertypes.ImageInspect, error)
}

type Containerd interface {
	CPullImage(image, namespaces string) (string, error)
	CIsImageExists(image, namespaces string) (bool, error)

}

// DockerClient is a wrapped layer of docker client for pixiu internal use.
type DockerClient struct {
	client *dockerapi.Client

	// timeout is the timeout of short running docker operations.
	timeout time.Duration
	// If no pulling progress is made before imagePullProgressDeadline, the image pulling will be cancelled.
	// Docker reports image progress for every 512kB block, so normally there shouldn't be too long interval
	// between progress updates.
	imagePullProgressDeadline time.Duration
}

type ContainerdClient struct {
	client *containerdapi.Client
}

// Make sure that DockerClient implemented the Interface.
var _ Interface = &DockerClient{}

var _ Containerd = &ContainerdClient{}

func getDockerClient(dockerEndpoint string) (*dockerapi.Client, error) {
	if len(dockerEndpoint) != 0 {
		klog.Infof("Connecting to docker on %s", dockerEndpoint)
		return dockerapi.NewClient(dockerEndpoint, "", nil, nil)
	}

	return dockerapi.NewClientWithOpts(dockerapi.FromEnv)
}

func ConnectToDockerOrDie(dockerEndpoint string, requestTimeout, imagePullProgressDeadline time.Duration) Interface {
	dockerEndpointclient, err := getDockerClient(dockerEndpoint)
	if err != nil {
		klog.Fatalf("Could not connect to docker: %v", err)
	}

	klog.V(2).Infof("Start docker client with request timeout=%v", requestTimeout)
	return newDockerClient(dockerEndpointclient, requestTimeout, imagePullProgressDeadline)
}

func newDockerClient(dockerClient *dockerapi.Client, requestTimeout, imagePullProgressDeadline time.Duration) Interface {
	if requestTimeout == 0 {
		requestTimeout = defaultTimeout
	}

	dc := &DockerClient{
		client:                    dockerClient,
		timeout:                   requestTimeout,
		imagePullProgressDeadline: imagePullProgressDeadline,
	}

	ctx, cancel := dc.getTimeoutContext()
	defer cancel()
	dockerClient.NegotiateAPIVersion(ctx)

	return dc
}

// operationTimeout is the error returned when the docker operations are timeout.
type operationTimeout struct {
	err error
}

func (e operationTimeout) Error() string {
	return fmt.Sprintf("operation timeout: %v", e.err)
}

// contextError checks the context, and returns error if the context is timeout.
func contextError(ctx context.Context) error {
	if ctx.Err() == context.DeadlineExceeded {
		return operationTimeout{err: ctx.Err()}
	}
	return ctx.Err()
}

// getTimeoutContext returns a new context with default request timeout
func (dc *DockerClient) getTimeoutContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), dc.timeout)
}

// getCancelableContext returns a new cancelable context. For long running requests without timeout, we use cancelable
// context to avoid potential resource leak, although the current implementation shouldn't leak resource.
func (dc *DockerClient) getCancelableContext() (context.Context, context.CancelFunc) {
	return context.WithCancel(context.Background())
}

func base64EncodeAuth(auth dockertypes.AuthConfig) (string, error) {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(auth); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(buf.Bytes()), nil
}

// getImageRef returns the image digest if exists, or else returns the image ID.
func (dc *DockerClient) getImageRef(image string) (string, error) {
	img, err := dc.InspectImageByRef(image)
	if err != nil {
		return "", err
	}
	if img == nil {
		return "", fmt.Errorf("unable to inspect image %s", image)
	}

	// Returns the digest if it exist.
	if len(img.RepoDigests) > 0 {
		return img.RepoDigests[0], nil
	}

	return img.ID, nil
}

func (dc *DockerClient) IsImageExists(image string, opts dockertypes.ImageListOptions) bool {
	ctx, cancel := dc.getCancelableContext()
	defer cancel()
	imageList, err := dc.client.ImageList(ctx, opts)
	if err != nil {
		return false
	}
	for _, imageName := range imageList {
		for _, repoTag := range imageName.RepoTags {
			if repoTag == image {
				return true
			}
		}
	}

	return false
}

func (dc *DockerClient) PullImage(image string, auth dockertypes.AuthConfig, opts dockertypes.ImagePullOptions) (string, error) {
	// RegistryAuth is the base64 encoded credentials for the registry
	authBase64, err := base64EncodeAuth(auth)
	if err != nil {
		return "", err
	}

	opts.RegistryAuth = authBase64

	ctx, cancel := dc.getCancelableContext()
	defer cancel()
	resp, err := dc.client.ImagePull(ctx, image, opts)
	if err != nil {
		return "", err
	}
	defer resp.Close()
	io.Copy(os.Stdout, resp)

	imageRef, err := dc.getImageRef(image)
	if err != nil {
		return "", err
	}

	return imageRef, nil
}

func (dc *DockerClient) RemoveImage(image string, opts dockertypes.ImageRemoveOptions) ([]dockertypes.ImageDeleteResponseItem, error) {
	ctx, cancel := dc.getTimeoutContext()
	defer cancel()
	resp, err := dc.client.ImageRemove(ctx, image, opts)
	if ctxErr := contextError(ctx); ctxErr != nil {
		return nil, ctxErr
	}
	if dockerapi.IsErrNotFound(err) {
		return nil, nil
	}

	return resp, nil
}

func getContainerdClient(containerdsock string) (*containerdapi.Client, error) {
	if len(containerdsock) != 0 {
		klog.Infof("connecting to container on %s", containerdsock)
		client, err := containerdapi.New(containerdsock)
		if err != nil {
			klog.Error(err)
		}
		defer client.Close()
		return client, err
	}
	return containerdapi.New("/run/containerd/containerd.sock")
}

func ConnectToContainerdOrDie(containerEndpoints string) Containerd {
	containerEndpointclinet, err := getContainerdClient(containerEndpoints)
	if err != nil {
		klog.Fatalf("Could not connect to container: %v", err)
	}
	cc := &ContainerdClient{
		client: containerEndpointclinet,
	}
	return cc
}

func (cc *ContainerdClient) CPullImage(image, namespaces string) (string, error) {
	ctx := cns.WithNamespace(context.Background(), namespaces)
	images, err := cc.client.Pull(ctx, image, containerdapi.WithPullUnpack)
	if err != nil {
		klog.Error(err)
	}
	return images.Name(), nil
}

func (cc *ContainerdClient) CIsImageExists(image, namespaces string) (bool, error) {
	ctx := cns.WithNamespace(context.Background(), namespaces)
	images, err := cc.client.ListImages(ctx)
	if err != nil {
		klog.Error(err)
	}
	for _, i := range images {
		if image != i.Name() {
			continue
		}
		return true, nil
	}
	return false, nil
}
//TODO
func (cc *ContainerdClient) CRemoveImage(image, namespaces string) (string, error) {
	return "", nil
}

// ImageNotFoundError is the error returned by InspectImage when image not found.
// Expose this to inject error in dockershim for testing.
type ImageNotFoundError struct {
	ID string
}

func (e ImageNotFoundError) Error() string {
	return fmt.Sprintf("no such image: %q", e.ID)
}

// IsImageNotFoundError checks whether the error is image not found error. This is exposed
// to share with dockershim.
func IsImageNotFoundError(err error) bool {
	_, ok := err.(ImageNotFoundError)
	return ok
}

func (dc *DockerClient) InspectImageByRef(imageRef string) (*dockertypes.ImageInspect, error) {
	ctx, cancel := dc.getTimeoutContext()
	defer cancel()
	resp, _, err := dc.client.ImageInspectWithRaw(ctx, imageRef)
	if ctxErr := contextError(ctx); ctxErr != nil {
		return nil, ctxErr
	}
	if err != nil {
		if dockerapi.IsErrNotFound(err) {
			err = ImageNotFoundError{ID: imageRef}
		}
		return nil, err
	}

	// TODO: need to check tag or sha match
	return &resp, nil
}
