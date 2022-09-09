package db

import (
	"gorm.io/gorm"

	"github.com/caoyingjunz/gopixiu/pkg/db/cloud"
	"github.com/caoyingjunz/gopixiu/pkg/db/user"
)

type ShareDaoFactory interface {
	User() user.UserInterface
	Cloud() cloud.CloudInterface
}

type shareDaoFactory struct {
	db *gorm.DB
}

func (f *shareDaoFactory) Cloud() cloud.CloudInterface {
	return cloud.NewCloud(f.db)
}

func (f *shareDaoFactory) User() user.UserInterface {
	return user.NewUser(f.db)
}

func NewDaoFactory(db *gorm.DB) ShareDaoFactory {
	return &shareDaoFactory{
		db: db,
	}
}
