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

package types

import (
	"time"

	appv1 "k8s.io/api/apps/v1"
	"k8s.io/api/core/v1"
)

const (
	timeLayout = "2006-01-02 15:04:05.999999999"
)

func (c *Cluster) SetId(i int64) {
	c.Id = i
}

func (o *KubeObject) SetReplicaSets(replicaSets []appv1.ReplicaSet) {
	o.lock.Lock()
	defer o.lock.Unlock()

	o.ReplicaSets = replicaSets
}

func (o *KubeObject) GetReplicaSets() []appv1.ReplicaSet {
	o.lock.Lock()
	defer o.lock.Unlock()

	return o.ReplicaSets
}

func (o *KubeObject) SetPods(pods []v1.Pod) {
	o.lock.Lock()
	defer o.lock.Unlock()

	o.Pods = pods
}

func (o *KubeObject) GetPods() []v1.Pod {
	o.lock.Lock()
	defer o.lock.Unlock()

	return o.Pods
}

func FormatTime(GmtCreate time.Time, GmtModified time.Time) TimeSpec {
	return TimeSpec{
		GmtCreate:   GmtCreate.Format(timeLayout),
		GmtModified: GmtModified.Format(timeLayout),
	}
}
