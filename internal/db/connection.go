package db

import (
	"errors"
	"fmt"
	"log"
	"os"
	"sync"

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

		log.Printf("Connecting to database: %s", dsn)
		dbInstance, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
		if err != nil {
			err = fmt.Errorf("%w: %w", ErrDatabaseURLInvalid, err)
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
		return nil, fmt.Errorf("failed to connect to database: %w", err)
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
