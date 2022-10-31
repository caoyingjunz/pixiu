package core

import (
	"context"
	
	"github.com/caoyingjunz/gopixiu/pkg/db"
)

type HealthzGetter interface {
	Healthz() HealthzInterface
}

// HealthzInterface 健康检查操作接口
type HealthzInterface interface {
	// GetHealthz main process healthz check
	GetHealthz(ctx context.Context) error
}

type healthz struct {
	app     *pixiu
	factory db.ShareDaoFactory
}

func newHealthz(c *pixiu) *healthz {
	return &healthz{
		app:     c,
		factory: c.factory,
	}
}

func (c *healthz) GetHealthz(ctx context.Context) error {

	return nil
}
