package model

import "time"

type ModelPricing struct {
	Id                     int     `json:"id"`
	ModelName              string  `json:"model_name" gorm:"type:varchar(255);index"`
	ProviderId             int     `json:"provider_id" gorm:"index"`
	QuotaType              int     `json:"quota_type"`
	ModelRatio             float64 `json:"model_ratio"`
	CompletionRatio        float64 `json:"completion_ratio"`
	ModelPrice             float64 `json:"model_price"`
	EnableGroups           string  `json:"enable_groups" gorm:"type:text"`
	SupportedEndpointTypes string  `json:"supported_endpoint_types" gorm:"type:text"`
	LastSynced             int64   `json:"last_synced"`
}

// UpsertModelPricing creates or updates a pricing record
func UpsertModelPricing(p *ModelPricing) error {
	var existing ModelPricing
	result := DB.Where("model_name = ? AND provider_id = ?", p.ModelName, p.ProviderId).First(&existing)
	if result.RowsAffected > 0 {
		p.Id = existing.Id
		if err := DB.Model(&existing).Updates(p).Error; err != nil {
			return err
		}
		invalidateModelRouteCaches()
		return nil
	}
	p.LastSynced = time.Now().Unix()
	if err := DB.Create(p).Error; err != nil {
		return err
	}
	invalidateModelRouteCaches()
	return nil
}

func GetModelPricingByProvider(providerId int) ([]*ModelPricing, error) {
	var pricing []*ModelPricing
	err := DB.Where("provider_id = ?", providerId).Find(&pricing).Error
	return pricing, err
}

func GetModelPricingByProviderAndModel(providerId int, modelName string) (*ModelPricing, error) {
	var pricing ModelPricing
	err := DB.Where("provider_id = ? AND model_name = ?", providerId, modelName).First(&pricing).Error
	if err != nil {
		return nil, err
	}
	return &pricing, nil
}

func GetAllModelPricing() ([]*ModelPricing, error) {
	var pricing []*ModelPricing
	err := DB.Find(&pricing).Error
	return pricing, err
}

// DeletePricingForProvider removes all pricing records for a provider
func DeletePricingForProvider(providerId int) error {
	if err := DB.Where("provider_id = ?", providerId).Delete(&ModelPricing{}).Error; err != nil {
		return err
	}
	invalidateModelRouteCaches()
	return nil
}
