package sqlite

import (
	"context"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/util/errors"
	"gorm.io/gorm"
	"time"
)

type user struct {
	db *gorm.DB
}

func NewUser(db *gorm.DB) *user {
	return &user{db: db}
}

func (u *user) Create(ctx context.Context, object *model.User) (*model.User, error) {
	now := time.Now()
	object.GmtCreate = now
	object.GmtModified = now

	if err := u.db.WithContext(ctx).Create(object).Error; err != nil {
		return nil, err
	}

	return object, nil
}

func (u *user) Update(ctx context.Context, uid int64, resourceVersion int64, updates map[string]interface{}) error {
	return nil
}

func (u *user) Delete(ctx context.Context, uid int64) error {
	return u.db.WithContext(ctx).Where("id = ?", uid).Delete(&model.User{}).Error
}

func (u *user) Get(ctx context.Context, uid int64) (*model.User, error) {
	var object model.User
	if err := u.db.WithContext(ctx).Where("id = ?", uid).First(&object).Error; err != nil {
		if errors.IsRecordNotFound(err) {
			return nil, nil
		}
		return nil, err
	}

	return &object, nil
}

// List 获取用户列表
// TODO: 暂时不做分页考虑
func (u *user) List(ctx context.Context) ([]model.User, error) {
	var objects []model.User
	if err := u.db.WithContext(ctx).Find(&objects).Error; err != nil {
		return nil, err
	}

	return objects, nil
}

func (u *user) Count(ctx context.Context) (int64, error) {
	var total int64
	if err := u.db.WithContext(ctx).Model(&model.User{}).Count(&total).Error; err != nil {
		return 0, err
	}

	return total, nil
}

func (u *user) GetUserByName(ctx context.Context, userName string) (*model.User, error) {
	var object model.User
	if err := u.db.WithContext(ctx).Where("name = ?", userName).First(&object).Error; err != nil {
		if errors.IsRecordNotFound(err) {
			return nil, nil
		}
		return nil, err
	}

	return &object, nil
}
