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

package core

import (
	"github.com/bndr/gojenkins"
	"k8s.io/client-go/kubernetes"

	"github.com/caoyingjunz/gopixiu/cmd/app/config"
	"github.com/caoyingjunz/gopixiu/pkg/db"
)

type CoreV1Interface interface {
	CicdGetter
	CloudGetter
	UserGetter
}

type pixiu struct {
	cfg        config.Config
	factory    db.ShareDaoFactory
	cicdDriver *gojenkins.Jenkins
	clientSets map[string]*kubernetes.Clientset
}

func (pixiu *pixiu) User() UserInterface {
	return newUser(pixiu)
}

func (pixiu *pixiu) Cicd() CicdInterface {
	return newCicd(pixiu)
}

func (pixiu *pixiu) Cloud() CloudInterface {
	return newCloud(pixiu)
}

func New(cfg config.Config, factory db.ShareDaoFactory, cicdDriver *gojenkins.Jenkins) CoreV1Interface {
	return &pixiu{
		cfg:        cfg,
		factory:    factory,
		cicdDriver: cicdDriver,
	}
}

const (
	timeLayout = "2006-01-02 15:04:05.999999999"
)
