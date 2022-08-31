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
	Create(ctx context.Context, typeUser *types.User) error
	Get(ctx context.Context, uid int64) (*types.User, error)
	List(ctx context.Context) ([]types.User, error)
	GetByName(ctx context.Context, name string) (*types.User, error)
	Delete(ctx context.Context, uid int64) error
	Update(ctx context.Context, typeUser *types.User) error
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

func (u *user) Delete(ctx context.Context, uid int64) error {
	return u.factory.User().Delete(ctx, uid)
}

func (u *user) Update(ctx context.Context, typeUser *types.User) error {
	modelUser := type2Model(typeUser)
	modelUser.Id = typeUser.Id
	if err := u.factory.User().Update(ctx, modelUser); err != nil {
		log.Logger.Errorf("failed to update %s user: %v", typeUser.Name, err)
		return err
	}
	return nil
}

func (u *user) Create(ctx context.Context, typeUser *types.User) error {
	// TODO 校验参数合法性
	_, err := u.factory.User().Create(ctx, type2Model(typeUser))
	if err != nil {
		log.Logger.Errorf("failed to create %s user: %v", typeUser.Name, err)
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

func (u *user) List(ctx context.Context) ([]types.User, error) {
	modelUsers, err := u.factory.User().List(ctx)
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

func (u *user) GetByName(ctx context.Context, name string) (*types.User, error) {
	modelUser, err := u.factory.User().GetByName(ctx, name)
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

func type2Model(u *types.User) *model.User {
	return &model.User{
		Name:        u.Name,
		Email:       u.Email,
		Description: u.Description,
		Status:      u.Status,
		Role:        u.Role,
		Password:    u.Password,
		// Extension:   obj.Extension,
	}
}
