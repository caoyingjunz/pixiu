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

package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"k8s.io/klog/v2"

	"github.com/caoyingjunz/pixiu/pkg/deployagent"
	"github.com/caoyingjunz/pixiu/pkg/types"
)

func main() {
	klog.InitFlags(nil)
	configPath := flag.String("config", "/etc/pixiu/agent.yaml", "配置文件路径")
	flag.Parse()

	cfg, err := deployagent.LoadConfig(*configPath)
	if err != nil {
		klog.Fatalf("Failed to load config: %v", err)
	}
	server, token, workRoot := cfg.Resolve()

	server = strings.TrimRight(strings.TrimSpace(server), "/")
	token = strings.TrimSpace(token)
	if server == "" || token == "" {
		klog.Fatalf("PIXIU_SERVER and PIXIU_DEPLOY_TOKEN are required (set via env or config file)")
	}
	if err := os.MkdirAll(workRoot, 0o755); err != nil {
		klog.Fatalf("Failed to init work root directory: %v", err)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	hostname, err := os.Hostname()
	if err != nil {
		klog.Fatalf("Failed to get hostname: %v", err)
	}
	ag := deployagent.New(server, token)

	klog.Infof("pixiu-deploy-agent %s starting, server=%s", deployagent.Version, server)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	var running bool

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err = ag.Heartbeat(hostname); err != nil {
				klog.Errorf("heartbeat failed: %v", err)
			}

			if running {
				continue
			}
			job, err := ag.Claim()
			if err != nil {
				klog.Errorf("claim failed: %v", err)
				continue
			}
			if job == nil {
				continue
			}
			klog.Infof("claimed job %d kind=%s action=%s", job.Id, job.Kind, job.Action)
			running = true
			go func(j *types.Job) {
				defer func() { running = false }()
				if err := deployagent.RunJob(ctx, ag, workRoot, j); err != nil {
					klog.Errorf("job %d failed: %v", j.Id, err)
					_ = ag.Report(j.Id, false, err.Error(), "")
				}
			}(job)
		}
	}
}
