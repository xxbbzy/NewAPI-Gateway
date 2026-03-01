package model

import "time"

type BackupUploadRetry struct {
	Id           int    `json:"id"`
	RunId        int    `json:"run_id" gorm:"index"`
	ArtifactPath string `json:"artifact_path" gorm:"type:text;not null"`
	RemotePath   string `json:"remote_path" gorm:"type:text"`
	RetryCount   int    `json:"retry_count" gorm:"index"`
	MaxRetries   int    `json:"max_retries"`
	NextRetryAt  int64  `json:"next_retry_at" gorm:"index"`
	LastError    string `json:"last_error" gorm:"type:text"`
	Status       string `json:"status" gorm:"type:varchar(16);index;not null"`
	CreatedAt    int64  `json:"created_at" gorm:"index"`
	UpdatedAt    int64  `json:"updated_at" gorm:"index"`
}

func (r *BackupUploadRetry) Insert() error {
	now := time.Now().Unix()
	if r.CreatedAt == 0 {
		r.CreatedAt = now
	}
	r.UpdatedAt = now
	if r.Status == "" {
		r.Status = "pending"
	}
	return DB.Create(r).Error
}

func (r *BackupUploadRetry) Update() error {
	r.UpdatedAt = time.Now().Unix()
	return DB.Model(r).Updates(r).Error
}

func GetDueBackupUploadRetries(now int64, limit int) ([]*BackupUploadRetry, error) {
	if limit <= 0 {
		limit = 20
	}
	var rows []*BackupUploadRetry
	err := DB.Where("status = ? AND next_retry_at <= ?", "pending", now).
		Order("id asc").
		Limit(limit).
		Find(&rows).Error
	return rows, err
}

func GetRecentBackupUploadRetries(limit int) ([]*BackupUploadRetry, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 500 {
		limit = 500
	}
	var rows []*BackupUploadRetry
	err := DB.Order("id desc").Limit(limit).Find(&rows).Error
	return rows, err
}

func CountPendingBackupRetries() (int64, error) {
	var count int64
	err := DB.Model(&BackupUploadRetry{}).Where("status = ?", "pending").Count(&count).Error
	return count, err
}
