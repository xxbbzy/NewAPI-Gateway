package model

import (
	"NewAPI-Gateway/common"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"
)

type AggregatedToken struct {
	Id                 int    `json:"id"`
	UserId             int    `json:"user_id" gorm:"index;not null"`
	Key                string `json:"key" gorm:"type:char(48);uniqueIndex"`
	Name               string `json:"name" gorm:"type:varchar(50)"`
	Status             int    `json:"status" gorm:"default:1"`
	ExpiredTime        int64  `json:"expired_time" gorm:"default:-1"`
	ModelLimitsEnabled bool   `json:"model_limits_enabled"`
	ModelLimits        string `json:"model_limits" gorm:"type:varchar(2048)"`
	AllowIps           string `json:"allow_ips" gorm:"type:text"`
	CreatedAt          int64  `json:"created_at"`
	AccessedAt         int64  `json:"accessed_at"`
}

const aggTokenKeyChars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
const maxAggTokenInsertAttempts = 8

var aggTokenKeyGenerator = generateAggTokenKey

func generateAggTokenKey() (string, error) {
	b := make([]byte, 48)
	max := big.NewInt(int64(len(aggTokenKeyChars)))
	for i := range b {
		n, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", err
		}
		b[i] = aggTokenKeyChars[n.Int64()]
	}
	return string(b), nil
}

func isAggTokenKeyCollisionError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "duplicate entry") ||
		strings.Contains(msg, "duplicate key value") ||
		strings.Contains(msg, "unique constraint failed") ||
		strings.Contains(msg, "duplicated key not allowed")
}

func GetAllUserAggTokens(userId int, startIdx int, num int) ([]*AggregatedToken, error) {
	var tokens []*AggregatedToken
	err := DB.Where("user_id = ?", userId).Order("id desc").Limit(num).Offset(startIdx).Find(&tokens).Error
	return tokens, err
}

func QueryUserAggTokens(userId int, startIdx int, num int) ([]*AggregatedToken, int64, error) {
	if startIdx < 0 {
		startIdx = 0
	}
	if num <= 0 {
		num = common.ItemsPerPage
	}

	base := DB.Model(&AggregatedToken{}).Where("user_id = ?", userId)
	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var tokens []*AggregatedToken
	err := base.Order("id desc").Limit(num).Offset(startIdx).Find(&tokens).Error
	return tokens, total, err
}

func GetAggTokenById(id int, userId int) (*AggregatedToken, error) {
	if id == 0 {
		return nil, errors.New("id 为空")
	}
	var token AggregatedToken
	err := DB.Where("id = ? AND user_id = ?", id, userId).First(&token).Error
	return &token, err
}

func GetAggTokenByKey(key string) (*AggregatedToken, error) {
	if key == "" {
		return nil, errors.New("key 为空")
	}
	var token AggregatedToken
	err := DB.Where(&AggregatedToken{Key: key}).First(&token).Error
	return &token, err
}

// ValidateAggToken validates the aggregated token and returns the token and user
func ValidateAggToken(key string) (*AggregatedToken, *User, error) {
	token, err := GetAggTokenByKey(key)
	if err != nil {
		return nil, nil, errors.New("无效的聚合令牌")
	}

	// Check status
	if token.Status != common.UserStatusEnabled {
		return nil, nil, errors.New("聚合令牌已被禁用")
	}

	// Check expiration
	if token.ExpiredTime != -1 && token.ExpiredTime < time.Now().Unix() {
		return nil, nil, errors.New("聚合令牌已过期")
	}

	// Check user
	user, err := GetUserById(token.UserId, false)
	if err != nil {
		return nil, nil, errors.New("令牌所属用户不存在")
	}
	if user.Status != common.UserStatusEnabled {
		return nil, nil, errors.New("令牌所属用户已被封禁")
	}

	// Update accessed time asynchronously
	go func(target *AggregatedToken) {
		if DB == nil || target == nil {
			return
		}
		_ = DB.Model(target).Update("accessed_at", time.Now().Unix()).Error
	}(token)

	return token, user, nil
}

// IsModelAllowed checks if the requested model is allowed by this token
func (t *AggregatedToken) IsModelAllowed(model string) bool {
	if !t.ModelLimitsEnabled || t.ModelLimits == "" {
		return true
	}

	requested := strings.TrimSpace(model)
	if requested == "" {
		return false
	}

	exactCandidates := make(map[string]bool)
	normalizedCandidates := make(map[string]bool)
	appendCandidate := func(candidate string) {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			return
		}
		exactCandidates[strings.ToLower(candidate)] = true
		if normalized := common.NormalizeModelName(candidate); normalized != "" {
			normalizedCandidates[normalized] = true
		}
	}

	appendCandidate(requested)
	if entry, ok, err := ResolveModelCatalogEntry(requested); err == nil && ok && entry != nil {
		appendCandidate(entry.CanonicalModel)
		for _, alias := range entry.Aliases {
			appendCandidate(alias)
		}
		for _, target := range entry.RouteTargets {
			appendCandidate(target)
		}
	}

	limits := strings.Split(t.ModelLimits, ",")
	for _, m := range limits {
		limit := strings.TrimSpace(m)
		if limit == "" {
			continue
		}
		if exactCandidates[strings.ToLower(limit)] {
			return true
		}
		limitNormalized := common.NormalizeModelName(limit)
		if limitNormalized != "" && normalizedCandidates[limitNormalized] {
			return true
		}
	}
	return false
}

// IsIPAllowed checks if the client IP is allowed
func (t *AggregatedToken) IsIPAllowed(ip string) bool {
	if t.AllowIps == "" {
		return true
	}
	ips := strings.Split(t.AllowIps, "\n")
	for _, allowed := range ips {
		allowed = strings.TrimSpace(allowed)
		if allowed != "" && allowed == ip {
			return true
		}
	}
	return false
}

func (t *AggregatedToken) Insert() error {
	t.CreatedAt = time.Now().Unix()
	var lastErr error
	for attempt := 0; attempt < maxAggTokenInsertAttempts; attempt++ {
		key, err := aggTokenKeyGenerator()
		if err != nil {
			return err
		}
		t.Key = key
		lastErr = DB.Create(t).Error
		if lastErr == nil {
			return nil
		}
		if !isAggTokenKeyCollisionError(lastErr) {
			return lastErr
		}
	}
	return fmt.Errorf("failed to generate unique aggregated token key after %d attempts: %w", maxAggTokenInsertAttempts, lastErr)
}

func (t *AggregatedToken) Update() error {
	return DB.Model(t).Select("name", "status", "expired_time", "model_limits_enabled",
		"model_limits", "allow_ips").Updates(t).Error
}

func (t *AggregatedToken) Delete() error {
	if t.Id == 0 {
		return errors.New("id 为空")
	}
	return DB.Delete(t).Error
}

func CountUserAggTokens(userId int) int64 {
	var count int64
	DB.Model(&AggregatedToken{}).Where("user_id = ?", userId).Count(&count)
	return count
}
