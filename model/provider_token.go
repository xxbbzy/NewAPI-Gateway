package model

import (
	"errors"
	"time"
)

type ProviderToken struct {
	Id              int    `json:"id"`
	ProviderId      int    `json:"provider_id" gorm:"index;not null"`
	UpstreamTokenId int    `json:"upstream_token_id"`
	SkKey           string `json:"sk_key" gorm:"type:varchar(256)"`
	Name            string `json:"name"`
	GroupName       string `json:"group_name" gorm:"type:varchar(64);index"`
	Status          int    `json:"status" gorm:"default:1"`
	Priority        int    `json:"priority" gorm:"default:0"`
	Weight          int    `json:"weight" gorm:"default:10"`
	RemainQuota     int64  `json:"remain_quota"`
	UnlimitedQuota  bool   `json:"unlimited_quota"`
	UsedQuota       int64  `json:"used_quota"`
	ModelLimits     string `json:"model_limits" gorm:"type:varchar(2048)"`
	LastSynced      int64  `json:"last_synced"`
	CreatedAt       int64  `json:"created_at"`
}

func GetProviderTokensByProviderId(providerId int) ([]*ProviderToken, error) {
	var tokens []*ProviderToken
	err := DB.Where("provider_id = ?", providerId).Order("id desc").Find(&tokens).Error
	return tokens, err
}

func GetEnabledProviderTokensByProviderId(providerId int) ([]*ProviderToken, error) {
	var tokens []*ProviderToken
	err := DB.Where("provider_id = ? AND status = 1", providerId).Find(&tokens).Error
	return tokens, err
}

func GetProviderTokenById(id int) (*ProviderToken, error) {
	if id == 0 {
		return nil, errors.New("id 为空")
	}
	var token ProviderToken
	err := DB.First(&token, "id = ?", id).Error
	return &token, err
}

func (pt *ProviderToken) Insert() error {
	pt.CreatedAt = time.Now().Unix()
	if err := DB.Create(pt).Error; err != nil {
		return err
	}
	invalidateModelRouteCaches()
	return nil
}

func (pt *ProviderToken) Update() error {
	if err := DB.Model(pt).Updates(pt).Error; err != nil {
		return err
	}
	invalidateModelRouteCaches()
	return nil
}

func (pt *ProviderToken) Delete() error {
	// Clean up related model_routes
	if err := DB.Where("provider_token_id = ?", pt.Id).Delete(&ModelRoute{}).Error; err != nil {
		return err
	}
	if err := DB.Delete(pt).Error; err != nil {
		return err
	}
	invalidateModelRouteCaches()
	return nil
}

// UpsertByUpstreamId creates or updates a provider token based on upstream token id + provider id
func UpsertProviderToken(pt *ProviderToken) error {
	var existing ProviderToken
	result := DB.Where("provider_id = ? AND upstream_token_id = ?", pt.ProviderId, pt.UpstreamTokenId).First(&existing)
	if result.RowsAffected > 0 {
		// Update existing
		pt.Id = existing.Id
		pt.CreatedAt = existing.CreatedAt
		if err := DB.Model(&existing).Updates(pt).Error; err != nil {
			return err
		}
		invalidateModelRouteCaches()
		return nil
	}
	// Create new
	pt.CreatedAt = time.Now().Unix()
	if err := DB.Create(pt).Error; err != nil {
		return err
	}
	invalidateModelRouteCaches()
	return nil
}

// DeleteProviderTokensNotInIds deletes tokens for a provider that are NOT in the given upstream token ID list
func DeleteProviderTokensNotInIds(providerId int, upstreamIds []int) error {
	if len(upstreamIds) == 0 {
		if err := DB.Where("provider_id = ?", providerId).Delete(&ProviderToken{}).Error; err != nil {
			return err
		}
		invalidateModelRouteCaches()
		return nil
	}
	if err := DB.Where("provider_id = ? AND upstream_token_id NOT IN (?)", providerId, upstreamIds).Delete(&ProviderToken{}).Error; err != nil {
		return err
	}
	invalidateModelRouteCaches()
	return nil
}

// CleanForResponse removes sensitive sk_key before sending to frontend
func (pt *ProviderToken) CleanForResponse() {
	if len(pt.SkKey) > 8 {
		pt.SkKey = pt.SkKey[:4] + "****" + pt.SkKey[len(pt.SkKey)-4:]
	}
}
