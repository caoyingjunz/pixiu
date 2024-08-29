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

package cluster

import (
	"sort"
	"strings"

	"github.com/caoyingjunz/pixiu/pkg/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

func (c *cluster) forQuery(objects []metav1.Object, queryOption types.QueryOption) []metav1.Object {
	if len(queryOption.LabelSelector) == 0 && len(queryOption.NameSelector) == 0 {
		return objects
	}

	queryObjects := make([]metav1.Object, 0)
	for _, object := range objects {
		// 标签搜索
		// TODO: 多个标签存在时，存在乱序时无法生效
		// 名称搜索
		if (len(queryOption.LabelSelector) != 0 && strings.Contains(labels.FormatLabels(object.GetLabels()), queryOption.LabelSelector)) || (len(queryOption.NameSelector) != 0 && strings.Contains(object.GetName(), queryOption.NameSelector)) {
			queryObjects = append(queryObjects, object)
		}
	}

	return queryObjects
}

func (c *cluster) forPage(objects []metav1.Object, pageOption types.PageRequest) []metav1.Object {
	if !pageOption.IsPaged() {
		return objects
	}
	offset, end, err := pageOption.Offset(len(objects))
	if err != nil {
		return nil
	}

	return objects[offset:end]
}

func (c *cluster) forSorted(objects []metav1.Object, namespace string) []metav1.Object {
	sort.SliceStable(objects, func(i, j int) bool {
		return objects[i].GetName() < objects[j].GetName()
	})
	// 全量获取 pod 时，以命名空间排序
	if len(namespace) == 0 {
		sort.SliceStable(objects, func(i, j int) bool {
			return objects[i].GetNamespace() < objects[j].GetNamespace()
		})
	}

	return objects
}

func (c *cluster) listObjects(objects []metav1.Object, namespace string, listOption types.ListOptions) (types.PageResponse, error) {
	objects = c.forQuery(objects, listOption.QueryOption)
	objects = c.forSorted(objects, namespace)
	return types.PageResponse{
		PageRequest: listOption.PageRequest,
		Total:       len(objects),
		Items:       c.forPage(objects, listOption.PageRequest),
	}, nil
}
