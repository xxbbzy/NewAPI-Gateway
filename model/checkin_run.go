package model

import (
	"errors"
	"time"

	"gorm.io/gorm"
)

type CheckinRun struct {
	Id             int    `json:"id"`
	TriggerType    string `json:"trigger_type" gorm:"type:varchar(32);index;not null"`
	Status         string `json:"status" gorm:"type:varchar(16);index;not null"`
	Timezone       string `json:"timezone" gorm:"type:varchar(64)"`
	ScheduledDate  string `json:"scheduled_date" gorm:"type:varchar(16);index"`
	StartedAt      int64  `json:"started_at" gorm:"index"`
	EndedAt        int64  `json:"ended_at"`
	TotalCount     int    `json:"total_count"`
	SuccessCount   int    `json:"success_count"`
	FailureCount   int    `json:"failure_count"`
	UncheckinCount int    `json:"uncheckin_count"`
	Message        string `json:"message" gorm:"type:text"`
	CreatedAt      int64  `json:"created_at" gorm:"index"`
}

type CheckinRunItem struct {
	Id           int    `json:"id"`
	RunId        int    `json:"run_id" gorm:"index;not null"`
	ProviderId   int    `json:"provider_id" gorm:"index;not null"`
	ProviderName string `json:"provider_name" gorm:"type:varchar(128);index"`
	Status       string `json:"status" gorm:"type:varchar(16);index;not null"`
	Message      string `json:"message" gorm:"type:text"`
	AutoDisabled bool   `json:"auto_disabled"`
	QuotaAwarded int64  `json:"quota_awarded"`
	CheckedAt    int64  `json:"checked_at" gorm:"index"`
	CreatedAt    int64  `json:"created_at" gorm:"index"`
}

func (r *CheckinRun) Insert() error {
	now := time.Now().Unix()
	if r.StartedAt == 0 {
		r.StartedAt = now
	}
	if r.CreatedAt == 0 {
		r.CreatedAt = now
	}
	return DB.Create(r).Error
}

func (r *CheckinRun) Update() error {
	if r.Id == 0 {
		return errors.New("id 为空")
	}
	return DB.Model(r).Updates(r).Error
}

func GetRecentCheckinRuns(limit int) ([]*CheckinRun, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 200 {
		limit = 200
	}
	var runs []*CheckinRun
	err := DB.Order("id desc").Limit(limit).Find(&runs).Error
	return runs, err
}

func GetLatestCheckinRun() (*CheckinRun, error) {
	var run CheckinRun
	err := DB.Order("id desc").First(&run).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &run, err
}

func ExistsCheckinRunByTriggerAndScheduledDate(triggerType string, scheduledDate string) (bool, error) {
	var count int64
	err := DB.Model(&CheckinRun{}).
		Where("trigger_type = ? AND scheduled_date = ?", triggerType, scheduledDate).
		Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func InsertCheckinRunItems(items []*CheckinRunItem) error {
	if len(items) == 0 {
		return nil
	}
	now := time.Now().Unix()
	for _, item := range items {
		if item.CheckedAt == 0 {
			item.CheckedAt = now
		}
		if item.CreatedAt == 0 {
			item.CreatedAt = now
		}
	}
	return DB.Create(&items).Error
}

func GetRecentCheckinRunItems(limit int) ([]*CheckinRunItem, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 500 {
		limit = 500
	}
	var items []*CheckinRunItem
	err := DB.Order("id desc").Limit(limit).Find(&items).Error
	return items, err
}

func GetLatestCheckinRunItemByProviderId(providerId int) (*CheckinRunItem, error) {
	var item CheckinRunItem
	err := DB.Where("provider_id = ?", providerId).Order("id desc").First(&item).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &item, err
}
