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
	"context"
	"sync"
	"time"

	"k8s.io/klog/v2"

	"github.com/caoyingjunz/pixiu/pkg/db"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
)

const (
	defaultQueueSize = 2048
	defaultWorkers   = 2
	writeTimeout     = 3 * time.Second
)

// Recorder 异步写入审计记录，避免审计落库拖慢请求。
type Recorder struct {
	factory db.ShareDaoFactory
	queue   chan *model.Audit

	mu     sync.RWMutex
	closed bool
	once   sync.Once
	done   chan struct{}
}

func NewRecorder(factory db.ShareDaoFactory) *Recorder {
	r := &Recorder{
		factory: factory,
		queue:   make(chan *model.Audit, defaultQueueSize),
		done:    make(chan struct{}),
	}

	var wg sync.WaitGroup
	wg.Add(defaultWorkers)
	for i := 0; i < defaultWorkers; i++ {
		go func() {
			defer wg.Done()
			r.run()
		}()
	}
	go func() {
		wg.Wait()
		close(r.done)
	}()
	return r
}

func (r *Recorder) Enqueue(record *model.Audit) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.closed {
		klog.Warningf("audit recorder is closed, drop record: %s", record.Path)
		return
	}
	select {
	case r.queue <- record:
	default:
		klog.Warningf("audit queue is full, drop record: %s", record.Path)
	}
}

// Close 停止接收新记录并等待队列中的记录写入完成，ctx 到期则提前返回。
func (r *Recorder) Close(ctx context.Context) error {
	r.once.Do(func() {
		r.mu.Lock()
		r.closed = true
		close(r.queue)
		r.mu.Unlock()
	})

	select {
	case <-r.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (r *Recorder) run() {
	for record := range r.queue {
		ctx, cancel := context.WithTimeout(context.Background(), writeTimeout)
		err := Write(ctx, r.factory, record)
		cancel()
		if err != nil {
			klog.Errorf("failed to create audit record [%s]: %v", record.String(), err)
		}
	}
}

// Write 使用调用方的 factory 写入审计，事务内调用时不切换连接或上下文。
func Write(ctx context.Context, factory db.ShareDaoFactory, record *model.Audit) error {
	if record.ResourceKind == "" {
		record.ResourceKind = ResourceKind(record.ObjectType)
	}
	if record.Action != model.AuditActionSystem {
		enrich(ctx, factory, record)
	}
	_, err := factory.Audit().Create(ctx, record)
	return err
}

func enrich(ctx context.Context, factory db.ShareDaoFactory, record *model.Audit) {
	if record.OperatorId == 0 && record.Operator != "" && record.Operator != "unknown" {
		if user, err := factory.User().GetUserByName(ctx, record.Operator); err == nil && user != nil {
			record.OperatorId = user.Id
		}
	}
	if record.ResourceId > 0 && record.Cluster == "" && record.ObjectType == model.ObjectCluster {
		if cluster, err := factory.Cluster().Get(ctx, record.ResourceId); err == nil && cluster != nil {
			record.Cluster = cluster.Name
			if record.ResourceName == "" {
				record.ResourceName = cluster.AliasName
				if record.ResourceName == "" {
					record.ResourceName = cluster.Name
				}
			}
		}
	}
	if record.ResourceId > 0 && (record.ObjectType == model.ObjectExternalProxy ||
		(record.ObjectType.IsDatasource() && record.ResourceName == "")) {
		if ds, err := factory.Datasource().Get(ctx, record.ResourceId); err == nil && ds != nil {
			if record.ObjectType == model.ObjectExternalProxy {
				switch ds.SubType {
				case model.DatasourceSubTypeNacos:
					record.ObjectType = model.ObjectNacos
				case model.DatasourceSubTypeES:
					record.ObjectType = model.ObjectES
				case model.DatasourceSubTypeRabbitMQ:
					record.ObjectType = model.ObjectRabbitMQ
				}
				if kind := ResourceKind(record.ObjectType); kind != "" {
					record.ResourceKind = kind
				}
			}
			if record.ResourceName == "" && record.ObjectType.IsDatasource() {
				record.ResourceName = ds.Name
			}
		}
	}
}
