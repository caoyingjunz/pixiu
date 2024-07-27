package audit

import (
	"context"
	"github.com/caoyingjunz/pixiu/api/server/errors"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/types"
	"github.com/gin-gonic/gin"
	"k8s.io/klog/v2"

	"github.com/caoyingjunz/pixiu/cmd/app/config"
	"github.com/caoyingjunz/pixiu/pkg/db"
)

type AuditGetter interface {
	Audit() Interface
}

type Interface interface {
	Delete(ctx context.Context, aid int64) error
	List(ctx context.Context) ([]types.Audit, error)
	Create(ctx *gin.Context, action, content string) error
	Get(ctx context.Context, aid int64) (*types.Audit, error)
}

type audit struct {
	cc      config.Config
	factory db.ShareDaoFactory
}

func (a *audit) Create(ctx *gin.Context, action, content string) error {
	ip := ctx.Value("ip")
	user := ctx.Value("user")
	_, ok1 := ip.(string)
	_, ok2 := user.(*model.User)

	//当获取ip和user失败时候跳过断言（debug模式获取不到user）
	var (
		//ip       string
		userName string
	)
	//todo 优化
	if !ok1 || !ok2 {
		userName = "unknown"
		goto next
	}
next:
	object := &model.Audit{
		Action:   action,
		Content:  content,
		Ip:       ip.(string),
		Operator: userName,
	}
	_, err := a.factory.Audit().Create(ctx, object)
	if err != nil {
		klog.Errorf("failed to create audit: %v", err)
		return errors.ErrServerInternal
	}
	return nil
}

func (a *audit) Get(ctx context.Context, aid int64) (*types.Audit, error) {
	object, err := a.factory.Audit().Get(ctx, aid)
	if err != nil {
		klog.Errorf("failed to get audit %d: %v", aid, err)
		return nil, errors.ErrServerInternal
	}
	if object == nil {
		return nil, errors.ErrAuditNotFound
	}
	return a.model2Type(object), nil
}

func (a *audit) List(ctx context.Context) ([]types.Audit, error) {
	objects, err := a.factory.Audit().List(ctx)
	if err != nil {
		klog.Errorf("failed to get tenants: %v", err)
		return nil, errors.ErrServerInternal
	}

	var ts []types.Audit
	for _, object := range objects {
		ts = append(ts, *a.model2Type(&object))
	}
	return ts, nil

}

func (a *audit) Delete(ctx context.Context, aid int64) error {
	_, err := a.factory.Audit().Delete(ctx, aid)
	if err != nil {
		klog.Errorf("failed to delete audit %d: %v", aid, err)
		return errors.ErrServerInternal
	}

	return nil
}

func (a *audit) model2Type(o *model.Audit) *types.Audit {
	return &types.Audit{
		PixiuMeta: types.PixiuMeta{
			Id:              o.Id,
			ResourceVersion: o.ResourceVersion,
		},
		TimeMeta: types.TimeMeta{
			GmtCreate:   o.GmtCreate,
			GmtModified: o.GmtModified,
		},
		Ip:       o.Ip,
		Action:   o.Action,
		Content:  o.Content,
		Operator: o.Operator,
	}
}

func NewAudit(cfg config.Config, f db.ShareDaoFactory) *audit {
	return &audit{
		cc:      cfg,
		factory: f,
	}
}
