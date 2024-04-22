package mysql

import (
	"github.com/caoyingjunz/pixiu/pkg/db/iface"
	"gorm.io/gorm"
)

type mysql struct {
	db *gorm.DB
}

func (m *mysql) Cluster() iface.ClusterInterface {
	return newCluster(m.db)
}

func (m *mysql) Tenant() iface.TenantInterface {
	return newTenant(m.db)
}

func (m *mysql) User() iface.UserInterface {
	return newUser(m.db)
}

func New(db *gorm.DB) (iface.ShareDaoFactory, error) {

	return &mysql{db: db}, nil
}
