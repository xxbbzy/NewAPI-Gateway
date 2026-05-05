package model

import (
	"NewAPI-Gateway/common"
	"errors"
	"strconv"
	"strings"
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
	HealthStatus             string `json:"health_status" gorm:"type:varchar(32);default:'unknown';index"`
	HealthCheckAt            int64  `json:"health_check_at"`
	HealthSuccessAt          int64  `json:"health_success_at"`
	HealthFailureAt          int64  `json:"health_failure_at"`
	HealthFailureReason      string `json:"health_failure_reason" gorm:"type:text"`
	HealthCooldownUntil      int64  `json:"health_cooldown_until"`
	ProxyEnabled             bool   `json:"proxy_enabled"`
	ProxyURL                 string `json:"proxy_url" gorm:"type:text"`
	PricingGroupRatio        string `json:"pricing_group_ratio" gorm:"type:text"`
	PricingUsableGroup       string `json:"pricing_usable_group" gorm:"type:text"`
	PricingSupportedEndpoint string `json:"pricing_supported_endpoint" gorm:"type:text"`
	ModelAliasMapping        string `json:"model_alias_mapping" gorm:"type:text"`
	Remark                   string `json:"remark" gorm:"type:text"`
	CreatedAt                int64  `json:"created_at"`
	HealthBlocked            bool   `json:"health_blocked" gorm:"-"`
	BalanceFreshness         string `json:"balance_freshness" gorm:"-"`
	RouteEligible            bool   `json:"route_eligible" gorm:"-"`
	RouteBlockReasons        []string `json:"route_block_reasons" gorm:"-"`
	ProxyURLRedacted         string `json:"proxy_url_redacted" gorm:"-"`
	LastCheckinStatus        string `json:"last_checkin_status" gorm:"-"`
	LastCheckinMessage       string `json:"last_checkin_message" gorm:"-"`
	LastCheckinQuotaAwarded  int64  `json:"last_checkin_quota_awarded" gorm:"-"`
	LastCheckinResultAt      int64  `json:"last_checkin_result_at" gorm:"-"`
}

func applyProviderReadProjection(db *gorm.DB) *gorm.DB {
	if db == nil || db.Dialector == nil || db.Dialector.Name() != "sqlite" {
		return db
	}
	// SQLite can hold text in INTEGER columns. Cast defensively to avoid scan failures
	// on historical/dirty rows restored from old snapshots.
	return db.Select(`
id,
name,
base_url,
access_token,
CAST(user_id AS INTEGER) AS user_id,
CAST(status AS INTEGER) AS status,
CAST(priority AS INTEGER) AS priority,
CAST(weight AS INTEGER) AS weight,
CAST(checkin_enabled AS INTEGER) AS checkin_enabled,
CAST(last_checkin_at AS INTEGER) AS last_checkin_at,
balance,
CAST(balance_updated AS INTEGER) AS balance_updated,
health_status,
CAST(health_check_at AS INTEGER) AS health_check_at,
CAST(health_success_at AS INTEGER) AS health_success_at,
CAST(health_failure_at AS INTEGER) AS health_failure_at,
health_failure_reason,
CAST(health_cooldown_until AS INTEGER) AS health_cooldown_until,
CAST(proxy_enabled AS INTEGER) AS proxy_enabled,
proxy_url,
pricing_group_ratio,
pricing_usable_group,
pricing_supported_endpoint,
model_alias_mapping,
remark,
CAST(created_at AS INTEGER) AS created_at`)
}

func GetAllProviders(startIdx int, num int) ([]*Provider, error) {
	var providers []*Provider
	err := applyProviderReadProjection(DB).Order("id desc").Limit(num).Offset(startIdx).Find(&providers).Error
	return providers, err
}

func QueryProviders(keyword string, routeFilter string, startIdx int, num int) ([]*Provider, int64, error) {
	if startIdx < 0 {
		startIdx = 0
	}
	if num <= 0 {
		num = common.ItemsPerPage
	}

	base := DB.Model(&Provider{})
	keyword = strings.TrimSpace(keyword)
	if keyword != "" {
		pattern := "%" + keyword + "%"
		if userID, err := strconv.Atoi(keyword); err == nil {
			base = base.Where(
				"name LIKE ? OR base_url LIKE ? OR remark LIKE ? OR user_id = ?",
				pattern,
				pattern,
				pattern,
				userID,
			)
		} else {
			base = base.Where(
				"name LIKE ? OR base_url LIKE ? OR remark LIKE ?",
				pattern,
				pattern,
				pattern,
			)
		}
	}
	filteredBase := applyProviderRouteFilterQuery(base, routeFilter, time.Now())
	var total int64
	if err := filteredBase.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var providers []*Provider
	if err := applyProviderReadProjection(filteredBase).Order("id desc").Limit(num).Offset(startIdx).Find(&providers).Error; err != nil {
		return nil, 0, err
	}

	for _, provider := range providers {
		if provider != nil {
			provider.applyRuntimeStateAt(time.Now())
		}
	}
	if startIdx >= int(total) {
		return []*Provider{}, total, nil
	}
	return providers, total, nil
}

func GetProviderById(id int) (*Provider, error) {
	if id == 0 {
		return nil, errors.New("id 为空")
	}
	var provider Provider
	err := applyProviderReadProjection(DB).First(&provider, "id = ?", id).Error
	return &provider, err
}

func GetEnabledProviders() ([]*Provider, error) {
	var providers []*Provider
	err := applyProviderReadProjection(DB).Where("status = ?", common.UserStatusEnabled).Find(&providers).Error
	return providers, err
}

func GetAutomatedSyncProviders() ([]*Provider, error) {
	var providers []*Provider
	err := applyProviderReadProjection(DB).Where("status = ?", common.UserStatusEnabled).Find(&providers).Error
	if err != nil {
		return nil, err
	}
	return filterAutomatedUsableProviders(providers), nil
}

func GetCheckinEnabledProviders() ([]*Provider, error) {
	var providers []*Provider
	err := applyProviderReadProjection(DB).Where("status = ? AND checkin_enabled = ?", common.UserStatusEnabled, true).Find(&providers).Error
	if err != nil {
		return nil, err
	}
	return filterAutomatedUsableProviders(providers), nil
}

func GetUncheckinProviders(dayStart int64) ([]*Provider, error) {
	var providers []*Provider
	err := applyProviderReadProjection(DB).Where(
		"status = ? AND checkin_enabled = ? AND last_checkin_at < ?",
		common.UserStatusEnabled,
		true,
		dayStart,
	).Order("id desc").Find(&providers).Error
	if err != nil {
		return nil, err
	}
	return filterAutomatedUsableProviders(providers), nil
}

func (p *Provider) Insert() error {
	p.CreatedAt = time.Now().Unix()
	if strings.TrimSpace(p.HealthStatus) == "" {
		p.HealthStatus = ProviderHealthStatusUnknown
	}
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
	now := time.Now()
	p.AccessToken = ""
	p.ProxyURLRedacted = RedactProxyURL(p.ProxyURL)
	p.ProxyURL = ""
	p.HealthStatus = normalizeProviderHealthStatus(p.HealthStatus)
	p.applyRuntimeStateAt(now)
}

// FindProviderByBaseURLAndUserID finds a provider by base_url + user_id combination.
// Returns nil, nil if not found.
func FindProviderByBaseURLAndUserID(baseURL string, userID int) (*Provider, error) {
	var provider Provider
	err := applyProviderReadProjection(DB).Where("base_url = ? AND user_id = ?", baseURL, userID).First(&provider).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &provider, nil
}

func CountProviders() int64 {
	var count int64
	DB.Model(&Provider{}).Count(&count)
	return count
}
