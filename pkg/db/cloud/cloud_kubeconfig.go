package cloud

import (
	"context"
	"time"

	"gorm.io/gorm"

	"github.com/caoyingjunz/gopixiu/pkg/db/model"
)

type KubeConfigInterface interface {
	Create(ctx context.Context, obj *model.KubeConfig) error
	Update(ctx context.Context, kid, resourceVersion int64, updates map[string]interface{}) error
	Delete(ctx context.Context, kid int64) error
	Get(ctx context.Context, kid int64) (*model.KubeConfig, error)
	List(ctx context.Context, cloudName string) ([]model.KubeConfig, error)
}

type kubeConfig struct {
	db *gorm.DB
}

func NewKubeConfig(db *gorm.DB) KubeConfigInterface {
	return &kubeConfig{db}
}

func (s *kubeConfig) Create(ctx context.Context, obj *model.KubeConfig) error {
	now := time.Now()
	obj.GmtCreate = now
	obj.GmtModified = now

	if err := s.db.Create(obj).Error; err != nil {
		return err
	}
	return nil
}

func (s *kubeConfig) Update(ctx context.Context, kid, resourceVersion int64, updates map[string]interface{}) error {
	updates["gmt_modified"] = time.Now()
	updates["resource_version"] = resourceVersion + 1

	f := s.db.Model(&model.KubeConfig{}).
		Where("id = ? and resource_version = ?", kid, resourceVersion).
		Updates(updates)
	if f.Error != nil {
		return f.Error
	}

	return nil
}

func (s *kubeConfig) Delete(ctx context.Context, kid int64) error {
	return s.db.
		Delete(&model.KubeConfig{}, kid).
		Error
}

func (s *kubeConfig) Get(ctx context.Context, kid int64) (*model.KubeConfig, error) {
	var obj model.KubeConfig
	if err := s.db.First(&obj, kid).Error; err != nil {
		return nil, err
	}

	return &obj, nil
}

func (s *kubeConfig) List(ctx context.Context, cloudName string) ([]model.KubeConfig, error) {
	var objs []model.KubeConfig
	if err := s.db.Where("cloud_name = ?", cloudName).Find(&objs).Error; err != nil {
		return nil, err
	}

	return objs, nil
}
