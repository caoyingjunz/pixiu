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

	"github.com/caoyingjunz/pixiu/pkg/db/model"
)

// TaskCache
// TODO: 临时实现，后续优化
type TaskCache struct {
	sync.RWMutex
	resultQueue map[int64]chan struct{}          // 存储每个任务的通道，用于循环从缓存中获取任务结果
	taskResult  map[int64]map[string]*model.Task // 存储每个任务的结果
}

func NewTaskCache() *TaskCache {
	return &TaskCache{
		resultQueue: map[int64]chan struct{}{},
		taskResult:  map[int64]map[string]*model.Task{},
	}
}

func (s *TaskCache) GetResultQueue(planId int64) (chan struct{}, bool) {
	s.RLock()
	defer s.RUnlock()

	t, ok := s.resultQueue[planId]
	return t, ok
}

func (s *TaskCache) GetPlanResult(planId int64) (map[string]*model.Task, bool) {
	s.RLock()
	defer s.RUnlock()

	t, ok := s.taskResult[planId]
	return t, ok
}
func (s *TaskCache) GetTaskResult(planId int64, taskName string) (*model.Task, bool) {
	s.RLock()
	defer s.RUnlock()

	t, ok := s.taskResult[planId][taskName]
	return t, ok
}
func (s *TaskCache) SetResultQueue(planId int64, result chan struct{}) {
	s.RLock()
	defer s.RUnlock()

	if s.resultQueue == nil {
		s.resultQueue = map[int64]chan struct{}{}
	}
	if s.taskResult[planId] == nil {
		//	初始化plan的taskResult
		s.taskResult[planId] = map[string]*model.Task{}
	}
	s.resultQueue[planId] = result
}

func (s *TaskCache) SetTaskResult(planId int64, data *model.Task) {
	s.RLock()
	defer s.RUnlock()
	// 如果channel没有启动，直接返回,不做操作
	if s.resultQueue == nil {
		return
	}
	// 如果没有plan的taskResult，初始化
	if s.taskResult[planId] == nil {
		//	初始化plan的taskResult
		s.taskResult[planId] = map[string]*model.Task{}
	}
	s.taskResult[planId][data.Name] = data
}

func (s *TaskCache) ClearPlanResult(planId int64) {
	s.RLock()
	defer s.RUnlock()

	delete(s.resultQueue, planId)
	//同步清空plan缓存
	delete(s.taskResult, planId)
}

func (s *TaskCache) Clear() {
	s.RLock()
	defer s.RUnlock()

	s.resultQueue = map[int64]chan struct{}{}
	s.taskResult = map[int64]map[string]*model.Task{}
}

func (s *TaskCache) CloseResultQueue(planId int64) {
	s.RLock()
	defer s.RUnlock()
	close(s.resultQueue[planId])
}
