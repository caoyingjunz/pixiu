package db

import (
	"context"
	"time"

	"gorm.io/gorm"

	"github.com/caoyingjunz/pixiu/pkg/db/model"
	utilerrors "github.com/caoyingjunz/pixiu/pkg/util/errors"
)

type BackupInterface interface {
	Plan() BackupPlanInterface
	Record() BackupRecordInterface
	Restore() RestoreTaskInterface
	RestoreRecord() RestoreRecordInterface
}

type BackupPlanInterface interface {
	Create(context.Context, *model.BackupPlan) (*model.BackupPlan, error)
	Get(context.Context, int64) (*model.BackupPlan, error)
	List(context.Context, ...Options) ([]model.BackupPlan, error)
	Update(context.Context, int64, int64, map[string]interface{}) error
	Delete(context.Context, int64) error
}

type BackupRecordInterface interface {
	Create(context.Context, *model.Backup) (*model.Backup, error)
	Get(context.Context, int64) (*model.Backup, error)
	List(context.Context, ...Options) ([]model.Backup, error)
	UpdateInternal(context.Context, int64, map[string]interface{}) error
	Delete(context.Context, int64) error
}

type RestoreTaskInterface interface {
	Create(context.Context, *model.RestoreTask) (*model.RestoreTask, error)
	Get(context.Context, int64) (*model.RestoreTask, error)
	List(context.Context, ...Options) ([]model.RestoreTask, error)
	UpdateInternal(context.Context, int64, map[string]interface{}) error
	Delete(context.Context, int64) error
}

type RestoreRecordInterface interface {
	Create(context.Context, *model.RestoreRecord) (*model.RestoreRecord, error)
	Get(context.Context, int64) (*model.RestoreRecord, error)
	List(context.Context, ...Options) ([]model.RestoreRecord, error)
	UpdateInternal(context.Context, int64, map[string]interface{}) error
	Delete(context.Context, int64) error
}

type backup struct{ db *gorm.DB }
type backupPlan struct{ db *gorm.DB }
type backupRecord struct{ db *gorm.DB }
type restoreTask struct{ db *gorm.DB }
type restoreRecord struct{ db *gorm.DB }

func newBackup(db *gorm.DB) BackupInterface     { return &backup{db: db} }
func (b *backup) Plan() BackupPlanInterface     { return &backupPlan{db: b.db} }
func (b *backup) Record() BackupRecordInterface { return &backupRecord{db: b.db} }
func (b *backup) Restore() RestoreTaskInterface { return &restoreTask{db: b.db} }
func (b *backup) RestoreRecord() RestoreRecordInterface {
	return &restoreRecord{db: b.db}
}

func (b *backupPlan) Create(ctx context.Context, o *model.BackupPlan) (*model.BackupPlan, error) {
	now := time.Now()
	o.GmtCreate = now
	o.GmtModified = now
	if err := b.db.WithContext(ctx).Create(o).Error; err != nil {
		return nil, err
	}
	return o, nil
}
func (b *backupPlan) Get(ctx context.Context, id int64) (*model.BackupPlan, error) {
	var o model.BackupPlan
	err := b.db.WithContext(ctx).First(&o, id).Error
	if utilerrors.IsRecordNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &o, nil
}
func (b *backupPlan) List(ctx context.Context, opts ...Options) ([]model.BackupPlan, error) {
	var o []model.BackupPlan
	tx := b.db.WithContext(ctx)
	for _, opt := range opts {
		tx = opt(tx)
	}
	return o, tx.Find(&o).Error
}
func (b *backupPlan) Update(ctx context.Context, id, version int64, updates map[string]interface{}) error {
	updates["gmt_modified"] = time.Now()
	updates["resource_version"] = version + 1
	r := b.db.WithContext(ctx).Model(&model.BackupPlan{}).Where("id = ? and resource_version = ?", id, version).Updates(updates)
	if r.Error != nil {
		return r.Error
	}
	if r.RowsAffected == 0 {
		return utilerrors.ErrRecordNotFound
	}
	return nil
}
func (b *backupPlan) Delete(ctx context.Context, id int64) error {
	r := b.db.WithContext(ctx).Delete(&model.BackupPlan{}, id)
	if r.Error != nil {
		return r.Error
	}
	if r.RowsAffected == 0 {
		return utilerrors.ErrRecordNotFound
	}
	return nil
}

func (b *backupRecord) Create(ctx context.Context, o *model.Backup) (*model.Backup, error) {
	now := time.Now()
	o.GmtCreate = now
	o.GmtModified = now
	if err := b.db.WithContext(ctx).Create(o).Error; err != nil {
		return nil, err
	}
	return o, nil
}
func (b *backupRecord) Get(ctx context.Context, id int64) (*model.Backup, error) {
	var o model.Backup
	err := b.db.WithContext(ctx).First(&o, id).Error
	if utilerrors.IsRecordNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &o, nil
}
func (b *backupRecord) List(ctx context.Context, opts ...Options) ([]model.Backup, error) {
	var o []model.Backup
	tx := b.db.WithContext(ctx)
	for _, opt := range opts {
		tx = opt(tx)
	}
	return o, tx.Find(&o).Error
}
func (b *backupRecord) UpdateInternal(ctx context.Context, id int64, updates map[string]interface{}) error {
	updates["gmt_modified"] = time.Now()
	return b.db.WithContext(ctx).Model(&model.Backup{}).Where("id = ?", id).Updates(updates).Error
}
func (b *backupRecord) Delete(ctx context.Context, id int64) error {
	return b.db.WithContext(ctx).Delete(&model.Backup{}, id).Error
}

func (b *restoreTask) Create(ctx context.Context, o *model.RestoreTask) (*model.RestoreTask, error) {
	now := time.Now()
	o.GmtCreate = now
	o.GmtModified = now
	if err := b.db.WithContext(ctx).Create(o).Error; err != nil {
		return nil, err
	}
	return o, nil
}
func (b *restoreTask) Get(ctx context.Context, id int64) (*model.RestoreTask, error) {
	var o model.RestoreTask
	err := b.db.WithContext(ctx).First(&o, id).Error
	if utilerrors.IsRecordNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &o, nil
}
func (b *restoreTask) List(ctx context.Context, opts ...Options) ([]model.RestoreTask, error) {
	var o []model.RestoreTask
	tx := b.db.WithContext(ctx)
	for _, opt := range opts {
		tx = opt(tx)
	}
	return o, tx.Find(&o).Error
}
func (b *restoreTask) UpdateInternal(ctx context.Context, id int64, updates map[string]interface{}) error {
	updates["gmt_modified"] = time.Now()
	return b.db.WithContext(ctx).Model(&model.RestoreTask{}).Where("id = ?", id).Updates(updates).Error
}
func (b *restoreTask) Delete(ctx context.Context, id int64) error {
	return b.db.WithContext(ctx).Delete(&model.RestoreTask{}, id).Error
}

func (b *restoreRecord) Create(ctx context.Context, o *model.RestoreRecord) (*model.RestoreRecord, error) {
	now := time.Now()
	o.GmtCreate = now
	o.GmtModified = now
	if err := b.db.WithContext(ctx).Create(o).Error; err != nil {
		return nil, err
	}
	return o, nil
}
func (b *restoreRecord) Get(ctx context.Context, id int64) (*model.RestoreRecord, error) {
	var o model.RestoreRecord
	err := b.db.WithContext(ctx).First(&o, id).Error
	if utilerrors.IsRecordNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &o, nil
}
func (b *restoreRecord) List(ctx context.Context, opts ...Options) ([]model.RestoreRecord, error) {
	var o []model.RestoreRecord
	tx := b.db.WithContext(ctx)
	for _, opt := range opts {
		tx = opt(tx)
	}
	return o, tx.Find(&o).Error
}
func (b *restoreRecord) UpdateInternal(ctx context.Context, id int64, updates map[string]interface{}) error {
	updates["gmt_modified"] = time.Now()
	return b.db.WithContext(ctx).Model(&model.RestoreRecord{}).Where("id = ?", id).Updates(updates).Error
}
func (b *restoreRecord) Delete(ctx context.Context, id int64) error {
	return b.db.WithContext(ctx).Delete(&model.RestoreRecord{}, id).Error
}
