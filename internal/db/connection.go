package db

import (
	"net/url"
	"os"
	"strings"
	"sync"

	svcerrors "project-serverless/internal/errors"
	"project-serverless/internal/logger"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var (
	dbInstance *gorm.DB
	once       sync.Once
	testDB     *gorm.DB
)

// Connect initializes the singleton GORM database instance.
func Connect() (*gorm.DB, error) {
	if os.Getenv("APP_ENV") == "test" {
		return nil, nil
	}

	var err error
	once.Do(func() {
		dsn := os.Getenv("DATABASE_URL")
		if dsn == "" {
			err = svcerrors.ErrDatabaseURLNotSet
			return
		}

		dsn = withPostgresSearchPath(dsn)

		host, dbname := maskDSNForLog(dsn)
		logger.Info("connecting_to_database", map[string]any{"host": host, "db": dbname})
		dbInstance, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
		if err != nil {
			err = svcerrors.Internal(svcerrors.ErrDatabaseURLInvalid.Message, err)
			return
		}

		sqlDB, sqlErr := dbInstance.DB()
		if sqlErr == nil {
			sqlDB.SetMaxOpenConns(5)
			sqlDB.SetMaxIdleConns(2)
			sqlDB.SetConnMaxLifetime(0)
		}
	})

	if err != nil {
		return nil, err
	}

	return dbInstance, nil
}

func ConnectWithDialector(dialector gorm.Dialector, config *gorm.Config) (*gorm.DB, error) {
	var err error
	once.Do(func() {
		dbInstance, err = gorm.Open(dialector, config)
	})

	if err != nil {
		return nil, svcerrors.Internal("failed to connect to database", err)
	}

	return dbInstance, nil
}

func GetDB() *gorm.DB {
	if testDB != nil {
		return testDB
	}
	if dbInstance == nil {
		panic("db: GetDB called before Connect — database connection not initialized")
	}
	return dbInstance
}

func SetDB(db *gorm.DB) {
	testDB = db
}

// withPostgresSearchPath sets libpq options so GORM table names "users" / "users_summary"
// resolve to write_model.users and read_model.users_summary (CQRS).
func withPostgresSearchPath(dsn string) string {
	u, err := url.Parse(dsn)
	if err != nil || u.Scheme == "" {
		return dsn
	}
	q := u.Query()
	if q.Get("options") != "" {
		return dsn
	}
	if strings.Contains(strings.ToLower(u.RawQuery), "search_path") {
		return dsn
	}
	q.Set("options", "-csearch_path=write_model,read_model,public")
	u.RawQuery = q.Encode()
	return u.String()
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
