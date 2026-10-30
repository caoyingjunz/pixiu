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

package types

// BatchOperationItem 批量操作成功/跳过的对象。
type BatchOperationItem struct {
	Key  int64  `json:"key"`
	Name string `json:"name,omitempty"`
}

// BatchOperationFailure 批量操作失败的明细。
type BatchOperationFailure struct {
	Key    int64  `json:"key"`
	Name   string `json:"name,omitempty"`
	Reason string `json:"reason"`
}

// BatchOperationResult 批量操作结果：单条失败隔离，不整体报错。
type BatchOperationResult struct {
	Succeeded []BatchOperationItem    `json:"succeeded"`
	Skipped   []BatchOperationItem    `json:"skipped"`
	Failed    []BatchOperationFailure `json:"failed"`
}
