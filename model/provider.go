package model

import (
	"NewAPI-Gateway/common"
	"errors"
	"time"

	"gorm.io/gorm"
)

type Provider struct {
	Id                       int    `json:"id"`
	Name                     string `json:"name" gorm:"index;not null"`
	BaseURL                  string `json:"base_url" gorm:"not null"`
	AccessToken              string `json:"access_token" gorm:"type:text"`
	UserID                   int    `json:"user_id"`
	Status                   int    `json:"status" gorm:"default:1"`
	Priority                 int    `json:"priority" gorm:"default:0"`
	Weight                   int    `json:"weight" gorm:"default:10"`
	CheckinEnabled           bool   `json:"checkin_enabled"`
	LastCheckinAt            int64  `json:"last_checkin_at"`
	Balance                  string `json:"balance"`
	BalanceUpdated           int64  `json:"balance_updated"`
	PricingGroupRatio        string `json:"pricing_group_ratio" gorm:"type:text"`
	PricingUsableGroup       string `json:"pricing_usable_group" gorm:"type:text"`
	PricingSupportedEndpoint string `json:"pricing_supported_endpoint" gorm:"type:text"`
	ModelAliasMapping        string `json:"model_alias_mapping" gorm:"type:text"`
	Remark                   string `json:"remark" gorm:"type:text"`
	CreatedAt                int64  `json:"created_at"`
	LastCheckinStatus        string `json:"last_checkin_status" gorm:"-"`
	LastCheckinMessage       string `json:"last_checkin_message" gorm:"-"`
	LastCheckinQuotaAwarded  int64  `json:"last_checkin_quota_awarded" gorm:"-"`
	LastCheckinResultAt      int64  `json:"last_checkin_result_at" gorm:"-"`
}

func GetAllProviders(startIdx int, num int) ([]*Provider, error) {
	var providers []*Provider
	err := DB.Order("id desc").Limit(num).Offset(startIdx).Find(&providers).Error
	return providers, err
}

func GetProviderById(id int) (*Provider, error) {
	if id == 0 {
		return nil, errors.New("id 为空")
	}
	var provider Provider
	err := DB.First(&provider, "id = ?", id).Error
	return &provider, err
}

func GetEnabledProviders() ([]*Provider, error) {
	var providers []*Provider
	err := DB.Where("status = ?", common.UserStatusEnabled).Find(&providers).Error
	return providers, err
}

func GetCheckinEnabledProviders() ([]*Provider, error) {
	var providers []*Provider
	err := DB.Where("status = ? AND checkin_enabled = ?", common.UserStatusEnabled, true).Find(&providers).Error
	return providers, err
}

func GetUncheckinProviders(dayStart int64) ([]*Provider, error) {
	var providers []*Provider
	err := DB.Where(
		"status = ? AND checkin_enabled = ? AND last_checkin_at < ?",
		common.UserStatusEnabled,
		true,
		dayStart,
	).Order("id desc").Find(&providers).Error
	return providers, err
}

func (p *Provider) Insert() error {
	p.CreatedAt = time.Now().Unix()
	return DB.Create(p).Error
}

func (p *Provider) Update() error {
	if err := DB.Model(p).Updates(p).Error; err != nil {
		return err
	}
	invalidateModelRouteCaches()
	return nil
}

func (p *Provider) Delete() error {
	if p.Id == 0 {
		return errors.New("id 为空")
	}
	err := DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("provider_id = ?", p.Id).Delete(&ProviderToken{}).Error; err != nil {
			return err
		}
		if err := tx.Where("provider_id = ?", p.Id).Delete(&ModelRoute{}).Error; err != nil {
			return err
		}
		if err := tx.Where("provider_id = ?", p.Id).Delete(&ModelPricing{}).Error; err != nil {
			return err
		}
		if err := tx.Delete(&Provider{}, "id = ?", p.Id).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}
	invalidateModelRouteCaches()
	return nil
}

func (p *Provider) UpdateBalance(balance string) {
	DB.Model(p).Updates(map[string]interface{}{
		"balance":         balance,
		"balance_updated": time.Now().Unix(),
	})
	invalidateModelRouteCaches()
}

func (p *Provider) UpdatePricingGroupRatio(groupRatio string) {
	DB.Model(p).Update("pricing_group_ratio", groupRatio)
	invalidateModelRouteCaches()
}

func (p *Provider) UpdatePricingUsableGroup(usableGroup string) {
	DB.Model(p).Update("pricing_usable_group", usableGroup)
	invalidateModelRouteCaches()
}

func (p *Provider) UpdatePricingSupportedEndpoint(supportedEndpoint string) {
	DB.Model(p).Update("pricing_supported_endpoint", supportedEndpoint)
	invalidateModelRouteCaches()
}

func (p *Provider) UpdateModelAliasMapping(modelAliasMapping string) {
	DB.Model(p).Update("model_alias_mapping", modelAliasMapping)
	invalidateModelRouteCaches()
}

func (p *Provider) UpdateCheckinTime() {
	DB.Model(p).Update("last_checkin_at", time.Now().Unix())
}

func (p *Provider) UpdateCheckinEnabled(enabled bool) error {
	if p.Id == 0 {
		return errors.New("id 为空")
	}
	return DB.Model(p).Update("checkin_enabled", enabled).Error
}

// CleanForResponse removes sensitive fields before sending to frontend
func (p *Provider) CleanForResponse() {
	p.AccessToken = ""
}

func CountProviders() int64 {
	var count int64
	DB.Model(&Provider{}).Count(&count)
	return count
}
