package model

import (
	"NewAPI-Gateway/common"
	"fmt"
	"strings"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func prepareAggregatedTokenSecurityDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:agg_token_security_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db failed: %v", err)
	}
	if err := db.AutoMigrate(&AggregatedToken{}, &User{}); err != nil {
		t.Fatalf("migrate db failed: %v", err)
	}
	return db
}

func TestGenerateAggTokenKeySecurityProperties(t *testing.T) {
	key, err := generateAggTokenKey()
	if err != nil {
		t.Fatalf("generate key failed: %v", err)
	}
	if len(key) != 48 {
		t.Fatalf("expected 48-char key, got len=%d key=%q", len(key), key)
	}
	for _, ch := range key {
		if !strings.ContainsRune(aggTokenKeyChars, ch) {
			t.Fatalf("unexpected key character %q in %q", ch, key)
		}
	}
}

func TestAggregatedTokenInsertRetriesCollision(t *testing.T) {
	originDB := DB
	DB = prepareAggregatedTokenSecurityDB(t)
	defer func() { DB = originDB }()

	collisionKey := strings.Repeat("a", 48)
	if err := DB.Create(&AggregatedToken{UserId: 1, Key: collisionKey, Name: "existing", Status: common.UserStatusEnabled, ExpiredTime: -1}).Error; err != nil {
		t.Fatalf("seed existing token failed: %v", err)
	}

	originGenerator := aggTokenKeyGenerator
	defer func() { aggTokenKeyGenerator = originGenerator }()
	calls := 0
	aggTokenKeyGenerator = func() (string, error) {
		calls++
		if calls == 1 {
			return collisionKey, nil
		}
		return generateAggTokenKey()
	}

	token := &AggregatedToken{UserId: 2, Name: "new-token", Status: common.UserStatusEnabled, ExpiredTime: -1}
	if err := token.Insert(); err != nil {
		t.Fatalf("insert with collision retry failed: %v", err)
	}
	if calls < 2 {
		t.Fatalf("expected collision retry path to call generator at least twice, calls=%d", calls)
	}
	if token.Key == collisionKey {
		t.Fatalf("expected final key to differ from colliding key")
	}
}

func TestValidateLegacyAggregatedTokenStillWorks(t *testing.T) {
	originDB := DB
	DB = prepareAggregatedTokenSecurityDB(t)
	defer func() { DB = originDB }()

	user := &User{Username: "legacy-user", Password: "hashed", Role: common.RoleCommonUser, Status: common.UserStatusEnabled}
	if err := DB.Create(user).Error; err != nil {
		t.Fatalf("seed user failed: %v", err)
	}
	legacyKey := strings.Repeat("b", 48)
	legacyToken := &AggregatedToken{UserId: user.Id, Key: legacyKey, Name: "legacy", Status: common.UserStatusEnabled, ExpiredTime: -1}
	if err := DB.Create(legacyToken).Error; err != nil {
		t.Fatalf("seed legacy token failed: %v", err)
	}

	validatedToken, validatedUser, err := ValidateAggToken(legacyKey)
	if err != nil {
		t.Fatalf("validate legacy token failed: %v", err)
	}
	if validatedToken == nil || validatedToken.Key != legacyKey {
		t.Fatalf("unexpected validated token: %+v", validatedToken)
	}
	if validatedUser == nil || validatedUser.Id != user.Id {
		t.Fatalf("unexpected validated user: %+v", validatedUser)
	}
}
