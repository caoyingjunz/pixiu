package db

<<<<<<< HEAD
import (
	"gorm.io/gorm"

	"github.com/caoyingjunz/gopixiu/pkg/db/cloud"
	"github.com/caoyingjunz/gopixiu/pkg/db/demo"
	"github.com/caoyingjunz/gopixiu/pkg/db/user"
)
=======
import "gorm.io/gorm"
>>>>>>> parent of 8acc52e (Add demo app api completed (get, post) (#26))

type ShareDaoFactory interface {
<<<<<<< HEAD
	User() user.UserInterface
	Demo() demo.DemoInterface
	Cloud() cloud.CloudInterface
=======
>>>>>>> parent of 8acc52e (Add demo app api completed (get, post) (#26))
}

type shareDaoFactory struct {
	db *gorm.DB
}

<<<<<<< HEAD
func (f *shareDaoFactory) Demo() demo.DemoInterface {
	return demo.NewDemo(f.db)
}

func (f *shareDaoFactory) Cloud() cloud.CloudInterface {
	return cloud.NewCloud(f.db)
}

func (f *shareDaoFactory) User() user.UserInterface {
	return user.NewUser(f.db)
}

=======
>>>>>>> parent of 8acc52e (Add demo app api completed (get, post) (#26))
func NewDaoFactory(db *gorm.DB) ShareDaoFactory {
	return &shareDaoFactory{
		db: db,
	}
}
