package sqlite

import (
	"gorm.io/gorm"
	"github.com/caoyingjunz/pixiu/pkg/db/iface"
)

type sqlite struct {
	db *gorm.DB
}

func (s *sqlite) Cluster() iface.ClusterInterface {
	return newCluster(s.db)
}

func (s *sqlite) Tenant() iface.TenantInterface {
	return newTenant(s.db)
}

func (s *sqlite) User() iface.UserInterface {
	return newUser(s.db)
}

func New(db *gorm.DB) (*sqlite, error) {
	return &sqlite{db: db}, nil
}
