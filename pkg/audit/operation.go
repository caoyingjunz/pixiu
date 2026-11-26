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

package audit

import (
	"net/http"

	"github.com/caoyingjunz/pixiu/pkg/db/model"
)

const (
	OperationCreate     = "create"
	OperationUpdate     = "update"
	OperationDelete     = "delete"
	OperationRestart    = "restart"
	OperationScale      = "scale"
	OperationExecute    = "execute"
	OperationBackup     = "backup"
	OperationRestore    = "restore"
	OperationImport     = "import"
	OperationGrant      = "grant"
	OperationLogin      = "login"
	OperationLogout     = "logout"
	OperationKill       = "kill"
	OperationUpload     = "upload"
	OperationDisconnect = "disconnect"
	OperationReconnect  = "reconnect"
)

func OperationForMethod(method string) string {
	switch method {
	case http.MethodPost:
		return OperationCreate
	case http.MethodPut, http.MethodPatch:
		return OperationUpdate
	case http.MethodDelete:
		return OperationDelete
	default:
		return ""
	}
}

var resourceKinds = map[model.ObjectType]string{
	model.ObjectCluster:        "Cluster",
	model.ObjectDatasource:     "Datasource",
	model.ObjectUser:           "User",
	model.ObjectAuth:           "Auth",
	model.ObjectTenant:         "Tenant",
	model.ObjectApp:            "App",
	model.ObjectPlan:           "Plan",
	model.ObjectMySQL:          "MySQL",
	model.ObjectPostgres:       "PostgreSQL",
	model.ObjectRedis:          "Redis",
	model.ObjectStorage:        "ObjectStorage",
	model.ObjectBackup:         "Backup",
	model.ObjectNacos:          "Nacos",
	model.ObjectES:             "Elasticsearch",
	model.ObjectRabbitMQ:       "RabbitMQ",
	model.ObjectContainerProxy: "Workload",
}

func ResourceKind(objectType model.ObjectType) string {
	return resourceKinds[objectType]
}
