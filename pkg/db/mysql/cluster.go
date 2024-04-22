package mysql

import (
	"context"
	"fmt"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/db/tx_func"
	"github.com/caoyingjunz/pixiu/pkg/util/errors"
	"gorm.io/gorm"
	"time"
)

type cluster struct {
	db *gorm.DB
}

func newCluster(db *gorm.DB) *cluster {
	return &cluster{db: db}
}

func (c *cluster) Create(ctx context.Context, object *model.Cluster, fns ...tx_func.TxFunc) (*model.Cluster, error) {
	now := time.Now()
	object.GmtCreate = now
	object.GmtModified = now

	if err := c.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(object).Error; err != nil {
			return err
		}

		for _, fn := range fns {
			if err := fn(); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return object, nil
}

func (c *cluster) Update(ctx context.Context, cid int64, resourceVersion int64, updates map[string]interface{}) error {
	// 系统维护字段
	updates["gmt_modified"] = time.Now()
	updates["resource_version"] = resourceVersion + 1

	f := c.db.WithContext(ctx).Model(&model.Cluster{}).Where("id = ? and resource_version = ?", cid, resourceVersion).Updates(updates)
	if f.Error != nil {
		return f.Error
	}
	if f.RowsAffected == 0 {
		return errors.ErrRecordNotUpdate
	}

	return nil
}

func (c *cluster) Delete(ctx context.Context, cid int64) (*model.Cluster, error) {
	// 仅当数据库支持回写功能时才能正常
	//if err := c.db.Clauses(clause.Returning{}).Where("id = ?", cid).Delete(&object).Error; err != nil {
	//	return nil, err
	//}
	object, err := c.Get(ctx, cid)
	if err != nil {
		return nil, err
	}
	if object == nil {
		return nil, nil
	}

	if object.Protected {
		return nil, fmt.Errorf("集群开启删除保护，不允许被删除")
	}
	if err = c.db.WithContext(ctx).Where("id = ?", cid).Delete(&model.Cluster{}).Error; err != nil {
		return nil, err
	}

	return object, nil
}

func (c *cluster) Get(ctx context.Context, cid int64) (*model.Cluster, error) {
	var object model.Cluster
	if err := c.db.WithContext(ctx).Where("id = ?", cid).First(&object).Error; err != nil {
		if errors.IsRecordNotFound(err) {
			return nil, nil
		}
		return nil, err
	}

	return &object, nil
}

func (c *cluster) List(ctx context.Context) ([]model.Cluster, error) {
	var cs []model.Cluster
	if err := c.db.WithContext(ctx).Find(&cs).Error; err != nil {
		return nil, err
	}

	return cs, nil
}

func (c *cluster) GetClusterByName(ctx context.Context, name string) (*model.Cluster, error) {
	var object model.Cluster
	if err := c.db.WithContext(ctx).Where("name = ?", name).First(&object).Error; err != nil {
		if errors.IsRecordNotFound(err) {
			return nil, nil
		}
		return nil, err
	}

	return &object, nil
}
