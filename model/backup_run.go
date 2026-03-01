package model

import (
	"errors"
	"time"

	"gorm.io/gorm"
)

type BackupRun struct {
	Id           int    `json:"id"`
	TriggerType  string `json:"trigger_type" gorm:"type:varchar(32);index;not null"`
	DBDriver     string `json:"db_driver" gorm:"type:varchar(16);index;not null"`
	Status       string `json:"status" gorm:"type:varchar(16);index;not null"`
	ArtifactPath string `json:"artifact_path" gorm:"type:text"`
	RemotePath   string `json:"remote_path" gorm:"type:text"`
	ArtifactSize int64  `json:"artifact_size"`
	DurationMs   int64  `json:"duration_ms"`
	ErrorMessage string `json:"error_message" gorm:"type:text"`
	StartedAt    int64  `json:"started_at" gorm:"index"`
	EndedAt      int64  `json:"ended_at"`
	CreatedAt    int64  `json:"created_at" gorm:"index"`
}

func (r *BackupRun) Insert() error {
	now := time.Now().Unix()
	if r.StartedAt == 0 {
		r.StartedAt = now
	}
	if r.CreatedAt == 0 {
		r.CreatedAt = now
	}
	return DB.Create(r).Error
}

func (r *BackupRun) Update() error {
	if r.Id == 0 {
		return errors.New("id 为空")
	}
	return DB.Model(r).Updates(r).Error
}

func GetRecentBackupRuns(limit int) ([]*BackupRun, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 500 {
		limit = 500
	}
	var runs []*BackupRun
	err := DB.Order("id desc").Limit(limit).Find(&runs).Error
	return runs, err
}

func GetBackupRunById(id int) (*BackupRun, error) {
	if id == 0 {
		return nil, errors.New("id 为空")
	}
	var run BackupRun
	err := DB.First(&run, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &run, nil
}

func GetLatestBackupRun() (*BackupRun, error) {
	var run BackupRun
	err := DB.Order("id desc").First(&run).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &run, nil
}
