package core

import (
	"context"

	"github.com/caoyingjunz/gopixiu/api/types"
	"github.com/caoyingjunz/gopixiu/cmd/app/config"
	"github.com/caoyingjunz/gopixiu/pkg/db"
	"github.com/caoyingjunz/gopixiu/pkg/db/model"
	"github.com/caoyingjunz/gopixiu/pkg/log"
)

type UserGetter interface {
	User() UserInterface
}

type UserInterface interface {
	Create(ctx context.Context, obj *types.User) error
	Get(ctx context.Context, uid int64) (*types.User, error)
	GetAll(ctx context.Context) ([]types.User, error)
	GetUserByName(ctx context.Context, name string) (*types.User, error)
	GetJWTKey() string
}

type user struct {
	ComponentConfig config.Config
	app             *pixiu
	factory         db.ShareDaoFactory
}

func newUser(c *pixiu) UserInterface {
	return &user{
		ComponentConfig: c.cfg,
		app:             c,
		factory:         c.factory,
	}
}

func (u *user) GetJWTKey() string {
	return u.ComponentConfig.Default.JWTKey
}

func (u *user) Create(ctx context.Context, obj *types.User) error {
	// TODO 校验参数合法性
	_, err := u.factory.User().Create(ctx, &model.User{
		Name:        obj.Name,
		Password:    obj.Password,
		Email:       obj.Email,
		Description: obj.Description,
		Status:      obj.Status,
		Role:        obj.Role,
		// Extension:   obj.Extension,
	})
	if err != nil {
		log.Logger.Errorf("failed to create %s user: %v", obj.Name, err)
		return err
	}

	return nil
}

func (u *user) Get(ctx context.Context, uid int64) (*types.User, error) {
	modelUser, err := u.factory.User().Get(ctx, uid)
	if err != nil {
		log.Logger.Errorf("failed to get %d user: %v", uid, err)
		return nil, err
	}

	return model2Type(modelUser), nil
}

func (u *user) GetAll(ctx context.Context) ([]types.User, error) {
	modelUsers, err := u.factory.User().GetAll(ctx)
	var typeUsers []types.User = make([]types.User, len(modelUsers))
	if err != nil {
		log.Logger.Errorf("failed to get user list: %v", err)
		return nil, err
	}

	for i, m := range modelUsers {
		typeUsers[i] = *model2Type(&m)
	}

	return typeUsers, nil
}

func (u *user) GetUserByName(ctx context.Context, name string) (*types.User, error) {
	modelUser, err := u.factory.User().GetUserByName(ctx, name)
	if err != nil {
		log.Logger.Errorf("failed to get user by name %s: %v", name, err)
		return nil, err
	}

	return model2Type(modelUser), nil
}

func model2Type(u *model.User) *types.User {
	return &types.User{
		Id:              u.Id,
		ResourceVersion: u.ResourceVersion,
		Name:            u.Name,
		Email:           u.Email,
		Description:     u.Description,
		Status:          u.Status,
		Role:            u.Role,
		Password:        u.Password,
		// Extension:   obj.Extension,
	}
}
