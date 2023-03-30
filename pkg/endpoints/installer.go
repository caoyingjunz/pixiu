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

package endpoints

import (
	"time"

	"github.com/emicklei/go-restful"
)

type APIInstaller struct {
	prefix            string // Path prefix where API resources are to be registered.
	minRequestTimeout time.Duration
}

// Struct capturing information about an action ("GET", "POST", "WATCH", "PROXY", etc).
type action struct {
	Verb   string               // Verb identifying the action ("GET", "POST", "WATCH", "PROXY", etc).
	Path   string               // The path of the action
	Params []*restful.Parameter // List of parameters associated with the action.
}

// Install handlers for API resources.
func (a *APIInstaller) Install() (*restful.WebService, error) {
	ws := a.newWebService()
	return ws, nil
}

// newWebService creates a new restful webservice with the api installer's prefix and version.
func (a *APIInstaller) newWebService() *restful.WebService {
	ws := new(restful.WebService)
	ws.Path(a.prefix)
	// a.prefix contains "prefix/group/version"
	ws.Doc("API at " + a.prefix)
	// Backwards compatibility, we accepted objects with empty content-type at V1.
	// If we stop using go-restful, we can default empty content-type to application/json on an
	// endpoint by endpoint basis
	ws.Consumes("*/*")

	return ws
}
