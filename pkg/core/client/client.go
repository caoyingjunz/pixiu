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

package client

import (
	"sync"

	"k8s.io/client-go/kubernetes"
)

type ClientsInterface interface {
	Add(key string, obj *kubernetes.Clientset)
	Update(key string, obj *kubernetes.Clientset)
	Delete(key string)
	Get(key string) *kubernetes.Clientset
	List() map[string]*kubernetes.Clientset
}

type cloudClient struct {
	lock  sync.RWMutex
	items map[string]*kubernetes.Clientset
}

func (cc *cloudClient) Add(key string, obj *kubernetes.Clientset) {
	cc.lock.Lock()
	defer cc.lock.Unlock()

	cc.items[key] = obj
}

func (cc *cloudClient) Update(key string, obj *kubernetes.Clientset) {
	cc.lock.Lock()
	defer cc.lock.Unlock()

	cc.items[key] = obj
}

func (cc *cloudClient) Delete(key string) {
	cc.lock.Lock()
	defer cc.lock.Unlock()
	if _, ok := cc.items[key]; ok {
		delete(cc.items, key)
	}
}

func (cc *cloudClient) Get(key string) *kubernetes.Clientset {
	cc.lock.Lock()
	defer cc.lock.Unlock()

	item, exists := cc.items[key]
	if !exists {
		return nil
	}
	return item
}

func (cc *cloudClient) List() map[string]*kubernetes.Clientset {
	cc.lock.Lock()
	defer cc.lock.Unlock()

	return cc.items
}

func NewCloudClients() ClientsInterface {
	return &cloudClient{items: map[string]*kubernetes.Clientset{}}
}
