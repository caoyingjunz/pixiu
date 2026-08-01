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

	"k8s.io/klog/v2"

	"github.com/caoyingjunz/pixiu/pkg/clusteragent"
)

func main() {
	klog.InitFlags(nil)
	flag.Parse()

	server := strings.TrimSpace(os.Getenv("PIXIU_SERVER"))
	token := strings.TrimSpace(os.Getenv("PIXIU_TOKEN"))
	if server == "" || token == "" {
		klog.Fatalf("PIXIU_SERVER and PIXIU_TOKEN are required")
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	cg := clusteragent.Agent{
		Server:   strings.TrimRight(server, "/"),
		Token:    token,
		Insecure: strings.EqualFold(os.Getenv("PIXIU_INSECURE"), "true"),
	}
	if err := cg.Run(ctx); err != nil {
		klog.Fatalf("agent run failed: %v", err)
	}
}
