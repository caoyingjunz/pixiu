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

package client

import (
	"context"
	"net"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/tunnel"
)

// DialContextFunc matches rest.Config.Dial.
type DialContextFunc func(ctx context.Context, network, address string) (net.Conn, error)

// ClusterSetOptions controls how ClusterSet is built.
type ClusterSetOptions struct {
	ClusterName string
	ConnectMode model.ConnectMode
	Dial        DialContextFunc
}

// NewClusterSetWithOptions builds a ClusterSet and optionally injects a custom Dialer
// (used for Agent reverse-tunnel clusters).
func NewClusterSetWithOptions(cfg string, opts ClusterSetOptions) (*ClusterSet, error) {
	kubeConfigBytes, err := ParseKubeConfigBytes(cfg)
	if err != nil {
		return nil, err
	}

	cs := &ClusterSet{}
	if err = cs.CompleteWithOptions(kubeConfigBytes, opts); err != nil {
		return nil, err
	}
	return cs, nil
}

func (cs *ClusterSet) Complete(cfg []byte) error {
	return cs.CompleteWithOptions(cfg, ClusterSetOptions{})
}

func (cs *ClusterSet) CompleteWithOptions(cfg []byte, opts ClusterSetOptions) error {
	var err error
	if cs.Config, err = clientcmd.RESTConfigFromKubeConfig(cfg); err != nil {
		return err
	}

	dial := opts.Dial
	if dial == nil && opts.ConnectMode == model.ConnectModeTunnel && opts.ClusterName != "" {
		if tm := tunnel.Default(); tm != nil {
			dial = tm.ClusterDialContext(opts.ClusterName)
		}
	}
	if dial != nil {
		cs.Config.Dial = dial
	}

	if cs.Client, err = kubernetes.NewForConfig(cs.Config); err != nil {
		return err
	}
	return nil
}
