package model

import (
	"NewAPI-Gateway/common"
	"fmt"
	"os"
	"strings"

	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

var DB *gorm.DB

func createRootAccountIfNeed() error {
	var user User
	//if user.Status != common.UserStatusEnabled {
	if err := DB.First(&user).Error; err != nil {
		common.SysLog("no user exists, create a root user for you: username is root, password is 123456")
		hashedPassword, err := common.Password2Hash("123456")
		if err != nil {
			return err
		}
		rootUser := User{
			Username:    "root",
			Password:    hashedPassword,
			Role:        common.RoleRootUser,
			Status:      common.UserStatusEnabled,
			DisplayName: "Root User",
		}
		DB.Create(&rootUser)
	}
	return nil
}

func CountTable(tableName string) (num int64) {
	DB.Table(tableName).Count(&num)
	return
}

func normalizeSQLDriver(driver string) string {
	switch strings.TrimSpace(strings.ToLower(driver)) {
	case "mysql":
		return "mysql"
	case "postgres", "postgresql":
		return "postgres"
	case "sqlite", "sqlite3":
		return "sqlite"
	default:
		return ""
	}
}

func detectSQLDriver(dsn string) string {
	dsnLower := strings.ToLower(strings.TrimSpace(dsn))
	if strings.HasPrefix(dsnLower, "postgres://") || strings.HasPrefix(dsnLower, "postgresql://") {
		return "postgres"
	}
	if strings.Contains(dsnLower, "dbname=") && strings.Contains(dsnLower, "user=") {
		return "postgres"
	}
	return "mysql"
}

func InitDB() (err error) {
	var db *gorm.DB

	dsn := strings.TrimSpace(os.Getenv("SQL_DSN"))
	configuredDriver := normalizeSQLDriver(os.Getenv("SQL_DRIVER"))
	if os.Getenv("SQL_DRIVER") != "" && configuredDriver == "" {
		return fmt.Errorf("unsupported SQL_DRIVER: %s (supported: mysql, postgres, sqlite)", os.Getenv("SQL_DRIVER"))
	}

	driver := configuredDriver
	if driver == "" {
		if dsn == "" {
			driver = "sqlite"
		} else {
			driver = detectSQLDriver(dsn)
		}
	}

	switch driver {
	case "mysql":
		if dsn == "" {
			return fmt.Errorf("SQL_DSN is required when SQL_DRIVER=mysql")
		}
		common.SysLog("using MySQL as database")
		db, err = gorm.Open(mysql.Open(dsn), &gorm.Config{
			PrepareStmt: true, // precompile SQL
		})
	case "postgres":
		if dsn == "" {
			return fmt.Errorf("SQL_DSN is required when SQL_DRIVER=postgres")
		}
		common.SysLog("using PostgreSQL as database")
		db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{
			PrepareStmt: true, // precompile SQL
		})
	case "sqlite":
		sqlitePath := common.SQLitePath
		if dsn != "" {
			sqlitePath = dsn
		}
		db, err = gorm.Open(sqlite.Open(sqlitePath), &gorm.Config{
			PrepareStmt: true, // precompile SQL
		})
		if configuredDriver == "" && dsn == "" {
			common.SysLog("SQL_DSN not set, using SQLite as database")
		} else {
			common.SysLog("using SQLite as database")
		}
	default:
		return fmt.Errorf("unsupported SQL driver: %s", driver)
	}

	if err == nil {
		DB = db
		err = db.AutoMigrate(&User{})
		if err != nil {
			return err
		}
		err = db.AutoMigrate(&Option{})
		if err != nil {
			return err
		}
		err = db.AutoMigrate(&Provider{})
		if err != nil {
			return err
		}
		err = db.AutoMigrate(&ProviderToken{})
		if err != nil {
			return err
		}
		err = db.AutoMigrate(&AggregatedToken{})
		if err != nil {
			return err
		}
		err = db.AutoMigrate(&ModelRoute{})
		if err != nil {
			return err
		}
		err = db.AutoMigrate(&ModelPricing{})
		if err != nil {
			return err
		}
		err = db.AutoMigrate(&UsageLog{})
		if err != nil {
			return err
		}
		err = db.AutoMigrate(&CheckinRun{})
		if err != nil {
			return err
		}
		err = db.AutoMigrate(&CheckinRunItem{})
		if err != nil {
			return err
		}
		err = createRootAccountIfNeed()
		return err
	} else {
		common.FatalLog(err)
	}
	return err
}

func CloseDB() error {
	sqlDB, err := DB.DB()
	if err != nil {
		return err
	}
	err = sqlDB.Close()
	return err
}
