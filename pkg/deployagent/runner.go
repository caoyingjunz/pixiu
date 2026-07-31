/*
Copyright 2026 The Pixiu Authors.

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

package deployagent

import (
	"context"
	"fmt"
	"io"
	"path/filepath"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"k8s.io/klog/v2"

	"github.com/caoyingjunz/pixiu/pkg/types"
)

func pullImage(ctx context.Context, ag *Agent, job *types.Job) error {
	klog.Infof("job %d: pulling image %s", job.Id, job.Image)
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		klog.Errorf("job %d: failed to create docker client: %v", job.Id, err)
		return err
	}
	defer cli.Close()

	_ = ag.Logs(job.Id, fmt.Sprintf("pulling image %s\n", job.Image))
	reader, err := cli.ImagePull(ctx, job.Image, image.PullOptions{})
	if err != nil {
		klog.Errorf("job %d: image pull failed: %v", job.Id, err)
		return err
	}
	defer reader.Close()

	_, _ = io.Copy(io.Discard, reader)
	klog.Infof("job %d: image %s pulled successfully", job.Id, job.Image)
	return ag.Report(job.Id, true, "image ready", "")
}

func runContainer(ctx context.Context, ag *Agent, workRoot string, job *types.Job) error {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return err
	}
	defer cli.Close()

	planDir := filepath.Join(workRoot, fmt.Sprintf("%d", job.PlanId))
	name := fmt.Sprintf("%s-%d", job.Action, job.PlanId)
	_ = removeContainerByName(ctx, cli, name)

	resp, err := cli.ContainerCreate(ctx, &container.Config{
		Image: job.Image,
		Env:   []string{fmt.Sprintf("COMMAND=%s", job.Action)},
	}, &container.HostConfig{
		Binds:       []string{fmt.Sprintf("%s:/configs", planDir)},
		NetworkMode: network.NetworkHost,
	}, nil, nil, name)
	if err != nil {
		return err
	}
	if err = cli.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		return err
	}

	statusCh, errCh := cli.ContainerWait(ctx, resp.ID, container.WaitConditionNotRunning)
	select {
	case err = <-errCh:
		if err != nil {
			return err
		}
	case st := <-statusCh:
		logs, _ := cli.ContainerLogs(ctx, resp.ID, container.LogsOptions{ShowStdout: true, ShowStderr: true})
		if logs != nil {
			b, _ := io.ReadAll(logs)
			_ = logs.Close()
			_ = ag.Logs(job.Id, string(b))
		}
		if st.StatusCode != 0 {
			return fmt.Errorf("container exit code %d", st.StatusCode)
		}
	}
	return ag.Report(job.Id, true, "ok", "")
}

func removeContainerByName(ctx context.Context, cli *client.Client, name string) error {
	cs, err := cli.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		return err
	}
	for _, c := range cs {
		for _, n := range c.Names {
			if n == "/"+name || n == name {
				timeout := 5
				_ = cli.ContainerStop(ctx, c.ID, container.StopOptions{Timeout: &timeout})
				return cli.ContainerRemove(ctx, c.ID, container.RemoveOptions{Force: true})
			}
		}
	}
	return nil
}
