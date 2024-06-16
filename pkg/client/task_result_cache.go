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

type TaskCache struct {
	sync.RWMutex
	quitQueue  map[int64]chan struct{}          // 存储每个任务的通道，用于循环从缓存中获取任务结果
	taskResult map[int64]map[string]*model.Task // 存储每个任务的结果
	taskQueue  map[int64]chan interface{}
}

func NewTaskCache() *TaskCache {
	return &TaskCache{
		quitQueue:  map[int64]chan struct{}{},
		taskResult: map[int64]map[string]*model.Task{},
		taskQueue:  map[int64]chan interface{}{},
	}
}
func (s *TaskCache) GetTaskQueue(planId int64) (chan interface{}, bool) {
	s.RLock()
	defer s.RUnlock()

	t, ok := s.taskQueue[planId]
	return t, ok
}

func (s *TaskCache) SetTaskQueue(planId int64, result chan interface{}) {
	s.RLock()
	defer s.RUnlock()
	s.taskQueue[planId] = result
}

func (s *TaskCache) GetQuitQueue(planId int64) (chan struct{}, bool) {
	s.RLock()
	defer s.RUnlock()

	t, ok := s.quitQueue[planId]
	return t, ok
}
func (s *TaskCache) SetQuitQueue(planId int64, result chan struct{}) {
	s.RLock()
	defer s.RUnlock()

	//	初始化plan的taskResult
	s.taskResult[planId] = map[string]*model.Task{}
	s.quitQueue[planId] = result
}

func (s *TaskCache) GetPlanResults(planId int64) (map[string]*model.Task, bool) {
	s.RLock()
	defer s.RUnlock()

	t, ok := s.taskResult[planId]
	return t, ok
}
func (s *TaskCache) GetTaskResults(planId int64, taskName string) (*model.Task, bool) {
	s.RLock()
	defer s.RUnlock()

	t, ok := s.taskResult[planId][taskName]
	return t, ok
}

func (s *TaskCache) SetTaskResults(planId int64, data *model.Task) {
	s.RLock()
	defer s.RUnlock()
	// 如果channel没有启动，直接返回,不做操作
	if s.quitQueue == nil {
		return
	}
	// 如果没有plan的taskResult，初始化
	if _, ok := s.taskResult[planId]; !ok {
		s.taskResult[planId] = map[string]*model.Task{}
	}
	s.taskResult[planId][data.Name] = data
}

// ClearPlanResults 根据planId清空plan的缓存
func (s *TaskCache) ClearPlanResults(planId int64) {
	s.RLock()
	defer s.RUnlock()
	delete(s.taskResult, planId)
}

func (s *TaskCache) Clear() {
	s.RLock()
	defer s.RUnlock()

	s.quitQueue = map[int64]chan struct{}{}
	s.taskResult = map[int64]map[string]*model.Task{}
}

func (s *TaskCache) CloseQuitQueue(planId int64) {
	s.Lock()
	defer s.Unlock()
	delete(s.quitQueue, planId)
}
