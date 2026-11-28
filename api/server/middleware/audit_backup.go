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

package middleware

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/caoyingjunz/pixiu/pkg/db"
)

type backupAuditHint struct {
	ResourceID   int64
	ResourceName string
	Cluster      string
}

func captureBackupAuditHint(c *gin.Context, factory db.ShareDaoFactory, body []byte) backupAuditHint {
	if c == nil || c.Request == nil || c.Request.URL == nil {
		return backupAuditHint{}
	}
	if !strings.HasPrefix(c.Request.URL.Path, "/pixiu/backup") {
		return backupAuditHint{}
	}

	ctx := c.Request.Context()
	hint := backupAuditHint{}
	if planID := parseIntParam(c, "planId"); planID > 0 {
		hint.ResourceID = planID
		if factory != nil {
			if plan, err := factory.Backup().Plan().Get(ctx, planID); err == nil && plan != nil {
				hint.ResourceName = plan.Name
				hint.Cluster = clusterNameByID(ctx, factory, plan.SourceClusterId)
			}
		}
	} else if backupID := parseIntParam(c, "backupId"); backupID > 0 {
		hint.ResourceID = backupID
		if factory != nil {
			if rec, err := factory.Backup().Record().Get(ctx, backupID); err == nil && rec != nil {
				hint.ResourceName = rec.ExternalName
				hint.Cluster = clusterNameByID(ctx, factory, rec.SourceClusterId)
			}
		}
	} else if restoreID := parseIntParam(c, "restoreId"); restoreID > 0 {
		hint.ResourceID = restoreID
		if factory != nil {
			if task, err := factory.Backup().Restore().Get(ctx, restoreID); err == nil && task != nil {
				hint.ResourceName = task.Name
				hint.Cluster = clusterNameByID(ctx, factory, task.TargetClusterId)
			}
		}
	}

	name, clusterID := extractBackupBodyHint(body)
	if hint.ResourceName == "" {
		hint.ResourceName = name
	}
	if hint.Cluster == "" && clusterID > 0 && factory != nil {
		hint.Cluster = clusterNameByID(ctx, factory, clusterID)
	}
	return hint
}

func extractBackupBodyHint(body []byte) (string, int64) {
	if len(body) == 0 {
		return "", 0
	}
	var obj struct {
		Name            string `json:"name"`
		SourceClusterID int64  `json:"source_cluster_id"`
		TargetClusterID int64  `json:"target_cluster_id"`
	}
	if err := json.Unmarshal(body, &obj); err != nil {
		return "", 0
	}
	clusterID := obj.SourceClusterID
	if clusterID <= 0 {
		clusterID = obj.TargetClusterID
	}
	return strings.TrimSpace(obj.Name), clusterID
}

func clusterNameByID(ctx context.Context, factory db.ShareDaoFactory, id int64) string {
	if factory == nil || id <= 0 {
		return ""
	}
	cluster, err := factory.Cluster().Get(ctx, id)
	if err != nil || cluster == nil {
		return ""
	}
	return cluster.Name
}
