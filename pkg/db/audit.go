package db

import (
	"context"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/util/errors"
	"gorm.io/gorm"
	"time"
)

type AuditInterface interface {
	List(ctx context.Context) ([]model.Audit, error)
	Get(ctx context.Context, id int64) (*model.Audit, error)
	Delete(ctx context.Context, id int64) (*model.Audit, error)
	Create(ctx context.Context, object *model.Audit) (*model.Audit, error)
}

type audit struct {
	db *gorm.DB
}

func newAudit(db *gorm.DB) AuditInterface {
	return &audit{db: db}
}

func (a *audit) Create(ctx context.Context, object *model.Audit) (*model.Audit, error) {
	now := time.Now()
	object.GmtCreate = now
	object.GmtModified = now
	if err := a.db.WithContext(ctx).Create(object).Error; err != nil {
		return nil, err
	}

	return object, nil
}

func (a *audit) Delete(ctx context.Context, aid int64) (*model.Audit, error) {
	object, err := a.Get(ctx, aid)
	if err != nil {
		return nil, err
	}
	if err := a.db.WithContext(ctx).Where("id = ?", aid).Delete(&model.Audit{}).Error; err != nil {
		return nil, err
	}
	return object, nil
}

func (a *audit) Get(ctx context.Context, aid int64) (*model.Audit, error) {
	var audit *model.Audit
	if err := a.db.WithContext(ctx).Where("id = ?", aid).First(audit).Error; err != nil {
		if errors.IsRecordNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return audit, nil
}

func (a *audit) List(ctx context.Context) ([]model.Audit, error) {
	var audits []model.Audit
	if err := a.db.WithContext(ctx).Find(&audits).Error; err != nil {
		return nil, err
	}
	return audits, nil
}
