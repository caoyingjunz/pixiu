package user

import (
	"context"
	"time"

	"github.com/caoyingjunz/gopixiu/pkg/db/model"
	"gorm.io/gorm"
)

type UserInterface interface {
	Get(ctx context.Context, uid int64) (*model.User, error)
	GetAll(ctx context.Context) ([]model.User, error)
	Create(ctx context.Context, obj *model.User) (*model.User, error)
}

type user struct {
	db *gorm.DB
}

func NewUser(db *gorm.DB) UserInterface {
	return &user{db}
}

func (u *user) Get(ctx context.Context, uid int64) (*model.User, error) {
	var res model.User
	if err := u.db.Where("id = ?", uid).First(&res).Error; err != nil {
		return nil, err
	}
	return &res, nil
}

func (u *user) GetAll(ctx context.Context) ([]model.User, error) {
	var users []model.User
	res := u.db.Find(&users)
	if res.Error != nil {
		return nil, res.Error
	}
	return users, nil
}

func (u *user) Create(ctx context.Context, obj *model.User) (*model.User, error) {
	now := time.Now()
	obj.GmtCreate = now
	obj.GmtModified = now
	if err := u.db.Create(obj).Error; err != nil {
		return nil, err
	}
	return obj, nil
}
