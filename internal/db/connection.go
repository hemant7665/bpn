package db

import (
	"errors"
	"net/url"
	"os"
	"sync"

	"project-serverless/internal/apperrors"
	"project-serverless/internal/logger"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var (
	ErrDatabaseURLNotSet  = errors.New("database URL is not set")
	ErrDatabaseURLInvalid = errors.New("database URL is invalid")

	dbInstance *gorm.DB
	once       sync.Once
	testDB     *gorm.DB
)

// Connect initializes the singleton GORM database instance.
// It reads DATABASE_URL from the environment — set to:
//
//	postgres://postgres:POSTGRES@localhost.localstack.cloud:4510/<dbname>?sslmode=disable
//
// when running against LocalStack RDS.
func Connect() (*gorm.DB, error) {
	if os.Getenv("APP_ENV") == "test" {
		return nil, nil
	}

	var err error
	once.Do(func() {
		dsn := os.Getenv("DATABASE_URL")
		if dsn == "" {
			err = ErrDatabaseURLNotSet
			return
		}

		host, dbname := maskDSNForLog(dsn)
		logger.Info("connecting_to_database", map[string]any{"host": host, "db": dbname})
		dbInstance, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
		if err != nil {
			err = apperrors.NewInternal(ErrDatabaseURLInvalid.Error(), err)
			return
		}

		// Configure connection pool.
		// Lambda containers are short-lived — keep pool small to avoid exhaustion.
		sqlDB, sqlErr := dbInstance.DB()
		if sqlErr == nil {
			sqlDB.SetMaxOpenConns(5)
			sqlDB.SetMaxIdleConns(2)
			sqlDB.SetConnMaxLifetime(0) // reuse until Lambda container dies
		}
	})

	if err != nil {
		return nil, err
	}

	return dbInstance, nil
}

// ConnectWithDialector initializes the singleton instance with a custom dialector.
// Useful for testing with sqlmock or alternative drivers.
func ConnectWithDialector(dialector gorm.Dialector, config *gorm.Config) (*gorm.DB, error) {
	var err error
	once.Do(func() {
		dbInstance, err = gorm.Open(dialector, config)
	})

	if err != nil {
		return nil, apperrors.NewInternal("failed to connect to database", err)
	}

	return dbInstance, nil
}

// GetDB returns the singleton database instance.
// Panics if called before Connect — this is intentional, it surfaces misconfiguration early.
func GetDB() *gorm.DB {
	if testDB != nil {
		return testDB
	}
	if dbInstance == nil {
		panic("db: GetDB called before Connect — database connection not initialized")
	}
	return dbInstance
}

// SetDB overrides the db instance, used in tests.
func SetDB(db *gorm.DB) {
	testDB = db
}

func maskDSNForLog(dsn string) (host string, dbname string) {
	parsed, err := url.Parse(dsn)
	if err != nil {
		return "unknown", "unknown"
	}

	host = parsed.Hostname()
	dbname = parsed.Path
	if len(dbname) > 0 && dbname[0] == '/' {
		dbname = dbname[1:]
	}

	if host == "" {
		host = "unknown"
	}
	if dbname == "" {
		dbname = "unknown"
	}
	return host, dbname
}
