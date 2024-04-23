package mysql

import (
	"gorm.io/gorm"
	"github.com/caoyingjunz/pixiu/pkg/db/iface"
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

func New(db *gorm.DB) (*mysql, error) {

	return &mysql{db: db}, nil
}
