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

package updater

import (
	"testing"

	"github.com/caoyingjunz/pixiu/pkg/db/model"
)

func TestDecideCheckStatus(t *testing.T) {
	tests := []struct {
		name          string
		currentStatus int
		available     bool
		sameVersion   bool
		want          int
	}{
		{name: "无记录发现有新版本", currentStatus: missingStatus, available: true, sameVersion: false, want: model.UpgradeStatusNotUpgraded},
		{name: "无记录且已是最新", currentStatus: missingStatus, available: false, sameVersion: false, want: model.UpgradeStatusCompleted},
		{name: "未升级发现有新版本", currentStatus: model.UpgradeStatusNotUpgraded, available: true, sameVersion: false, want: model.UpgradeStatusNotUpgraded},
		{name: "失败后出现更新的版本重置为未升级", currentStatus: model.UpgradeStatusFailed, available: true, sameVersion: false, want: model.UpgradeStatusNotUpgraded},
		{name: "同一版本失败保持失败态", currentStatus: model.UpgradeStatusFailed, available: true, sameVersion: true, want: model.UpgradeStatusFailed},
		{name: "已完成发现新版本重置为未升级", currentStatus: model.UpgradeStatusCompleted, available: true, sameVersion: false, want: model.UpgradeStatusNotUpgraded},
		{name: "未升级且已是最新标记完成", currentStatus: model.UpgradeStatusNotUpgraded, available: false, sameVersion: false, want: model.UpgradeStatusCompleted},
		{name: "失败且已是最新标记完成", currentStatus: model.UpgradeStatusFailed, available: false, sameVersion: true, want: model.UpgradeStatusCompleted},
		{name: "已完成且仍是最新保持完成", currentStatus: model.UpgradeStatusCompleted, available: false, sameVersion: true, want: model.UpgradeStatusCompleted},
		{name: "升级进行中且无新版本保持原状态", currentStatus: model.UpgradeStatusPending, available: false, sameVersion: true, want: model.UpgradeStatusPending},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := decideCheckStatus(tt.currentStatus, tt.available, tt.sameVersion); got != tt.want {
				t.Fatalf("decideCheckStatus(%d, %v, %v) = %d, want %d",
					tt.currentStatus, tt.available, tt.sameVersion, got, tt.want)
			}
		})
	}
}

func TestCheckStatusLastError(t *testing.T) {
	failedRecord := &model.UpgradeRecord{LastError: "拉取镜像失败"}

	tests := []struct {
		name   string
		status int
		record *model.UpgradeRecord
		want   string
	}{
		{name: "失败态保留上一轮失败原因", status: model.UpgradeStatusFailed, record: failedRecord, want: "拉取镜像失败"},
		{name: "失败态无记录时为空", status: model.UpgradeStatusFailed, record: nil, want: ""},
		{name: "未升级态清空失败原因", status: model.UpgradeStatusNotUpgraded, record: failedRecord, want: ""},
		{name: "完成态清空失败原因", status: model.UpgradeStatusCompleted, record: failedRecord, want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := checkStatusLastError(tt.status, tt.record); got != tt.want {
				t.Fatalf("checkStatusLastError(%d, %+v) = %q, want %q", tt.status, tt.record, got, tt.want)
			}
		})
	}
}

func TestMatchUpgradeTarget(t *testing.T) {
	target := &model.UpgradeRecord{
		LatestVersion: "v2.0.2",
		Image:         "registry.example.com/pixiu/pixiu:v2.0.2",
	}

	tests := []struct {
		name         string
		currentImage string
		buildVersion string
		record       *model.UpgradeRecord
		want         bool
	}{
		{
			name:         "运行镜像 tag 与目标版本一致",
			currentImage: "registry.example.com/pixiu/pixiu:v2.0.2",
			record:       target,
			want:         true,
		},
		{
			name:         "运行镜像 tag 不带 v 前缀仍视为一致",
			currentImage: "registry.example.com/pixiu/pixiu:2.0.2",
			record:       target,
			want:         true,
		},
		{
			name:         "镜像 tag 无法解析但运行镜像与目标镜像完全相等",
			currentImage: "registry.example.com/pixiu/pixiu:latest-snapshot",
			record: &model.UpgradeRecord{
				LatestVersion: "v2.0.2",
				Image:         "registry.example.com/pixiu/pixiu:latest-snapshot",
			},
			want: true,
		},
		{
			name:         "运行镜像为 latest 但编译版本与目标一致",
			currentImage: "registry.example.com/pixiu/pixiu:latest",
			buildVersion: "v2.0.2",
			record:       target,
			want:         true,
		},
		{
			name:         "编译版本匹配但运行镜像不可解析",
			currentImage: "",
			buildVersion: "2.0.2",
			record:       target,
			want:         true,
		},
		{
			name:         "均不匹配",
			currentImage: "registry.example.com/pixiu/pixiu:v2.0.1",
			buildVersion: "v2.0.1",
			record:       target,
			want:         false,
		},
		{
			name:         "记录为空",
			currentImage: "registry.example.com/pixiu/pixiu:v2.0.2",
			record:       nil,
			want:         false,
		},
		{
			name:         "空镜像且空编译版本",
			currentImage: "",
			record:       target,
			want:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := matchUpgradeTarget(tt.currentImage, tt.buildVersion, tt.record); got != tt.want {
				t.Fatalf("matchUpgradeTarget(%q, %q, %+v) = %v, want %v",
					tt.currentImage, tt.buildVersion, tt.record, got, tt.want)
			}
		})
	}
}
