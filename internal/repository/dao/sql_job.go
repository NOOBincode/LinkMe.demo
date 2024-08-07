package dao

import (
	"context"
	"gorm.io/gorm"
	"time"
)

const (
	// jobStatusWaiting 任务处于等待状态
	jobStatusWaiting = iota
	// jobStatusRunning 任务正在运行
	jobStatusRunning
	// jobStatusPaused 任务已暂停
	jobStatusPaused
)

// JobDAO 定义了任务数据访问对象接口
type JobDAO interface {
	Preempt(ctx context.Context) (Job, error)
	Release(ctx context.Context, jobId int64) error
	UpdateTime(ctx context.Context, id int64) error
	UpdateNextTime(ctx context.Context, id int64, t time.Time) error
}

// jobDAO 实现了 JobDAO 接口
type jobDAO struct {
	db *gorm.DB
}

type Job struct {
	Id          int64  `gorm:"primaryKey,autoIncrement"`               // 任务ID，主键，自增
	Name        string `gorm:"type:varchar(128);unique"`               // 任务名称，唯一
	Executor    string `gorm:"type:varchar(255)"`                      // 执行者，执行该任务的实体
	Expression  string `gorm:"type:varchar(255)"`                      // 调度表达式，用于描述任务的调度时间
	Cfg         string `gorm:"type:text"`                              // 配置，任务的具体配置信息
	Status      int    `gorm:"type:int"`                               // 任务状态，用于标识任务当前的状态（如启用、禁用等）
	Version     int    `gorm:"type:int"`                               // 版本号，用于乐观锁控制并发更新
	NextTime    int64  `gorm:"index"`                                  // 下次执行时间，Unix时间戳
	CreateTime  int64  `gorm:"column:created_at;type:bigint;not null"` // 创建时间，Unix时间戳
	UpdatedTime int64  `gorm:"column:updated_at;type:bigint;not null"` // 更新时间，Unix时间戳
}

// NewJobDAO 创建并初始化 jobDAO 实例
func NewJobDAO(db *gorm.DB) JobDAO {
	return &jobDAO{
		db: db,
	}
}

// Preempt 抢占一个等待状态的任务
func (dao *jobDAO) Preempt(ctx context.Context) (Job, error) {
	db := dao.db.WithContext(ctx)
	for {
		var j Job
		now := time.Now().UnixMilli()
		// 查找一个等待状态且下一次执行时间小于当前时间的任务
		err := db.Where("status = ? AND next_time < ?", jobStatusWaiting, now).First(&j).Error
		if err != nil {
			return j, err
		}
		// 尝试更新任务的状态和版本
		result := db.Model(&Job{}).Where("id = ? AND version = ?", j.Id, j.Version).Updates(map[string]any{
			"status":     jobStatusRunning,
			"version":    j.Version + 1,
			"updated_at": now,
		})
		if result.Error != nil {
			return Job{}, result.Error
		}
		if result.RowsAffected == 0 {
			// 如果没有抢到任务，继续循环
			continue
		}
		return j, nil
	}
}

// Release 释放一个正在运行的任务，将其状态重置为等待状态
func (dao *jobDAO) Release(ctx context.Context, jobId int64) error {
	now := time.Now().UnixMilli()
	return dao.db.WithContext(ctx).Model(&Job{}).Where("id = ?", jobId).Updates(map[string]any{
		"status":     jobStatusWaiting,
		"updated_at": now,
	}).Error
}

// UpdateTime 更新任务的更新时间
func (dao *jobDAO) UpdateTime(ctx context.Context, id int64) error {
	now := time.Now().UnixMilli()
	return dao.db.WithContext(ctx).Model(&Job{}).Where("id = ?", id).Updates(map[string]any{
		"updated_at": now,
	}).Error
}

// UpdateNextTime 更新任务的下次执行时间
func (dao *jobDAO) UpdateNextTime(ctx context.Context, id int64, t time.Time) error {
	now := time.Now().UnixMilli()
	return dao.db.WithContext(ctx).Model(&Job{}).Where("id = ?", id).Updates(map[string]any{
		"updated_at": now,
		"next_time":  t.UnixMilli(),
	}).Error
}
