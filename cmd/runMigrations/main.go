package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

func main() {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		panic("DATABASE_URL is required")
	}

	migrationDir := os.Getenv("MIGRATION_DIR")
	if migrationDir == "" {
		migrationDir = "scripts/migrate"
	}

	absDir, err := filepath.Abs(migrationDir)
	if err != nil {
		panic(fmt.Sprintf("failed to resolve migration dir: %v", err))
	}

	m, err := migrate.New("file://"+filepath.ToSlash(absDir), dbURL)
	if err != nil {
		panic(fmt.Sprintf("failed to initialize migration: %v", err))
	}
	defer func() {
		srcErr, dbErr := m.Close()
		if srcErr != nil || dbErr != nil {
			panic(fmt.Sprintf("failed to close migration resources: src=%v db=%v", srcErr, dbErr))
		}
	}()

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		panic(fmt.Sprintf("failed to run migrations: %v", err))
	}

	fmt.Println("Migrations applied successfully.")
}
