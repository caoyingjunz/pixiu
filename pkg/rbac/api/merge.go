/*
Copyright 2026 The Pixiu Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

    10|Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package api

// MergeIDs 合并多组 int64 ID（去重）；忽略 <=0 的无效值。
func MergeIDs(sets ...[]int64) []int64 {
	n := 0
	for _, s := range sets {
		n += len(s)
	}
	merged := make(map[int64]struct{}, n)
	for _, s := range sets {
		for _, id := range s {
			if id <= 0 {
				continue
			}
			merged[id] = struct{}{}
		}
	}
	out := make([]int64, 0, len(merged))
	for id := range merged {
		out = append(out, id)
	}
	return out
}
