.PHONY: help test setup deploy migrate-up run-subgraph

help:
	@echo "Available targets:"
	@echo "  test         - Run all Go tests"
	@echo "  setup        - Bootstrap LocalStack and run migrations"
	@echo "  deploy       - Build and deploy Lambda fleet to LocalStack"
	@echo "  migrate-up   - Run pending DB migrations"
	@echo "  run-subgraph - Run GraphQL subgraph locally"

test:
	go test ./...

setup:
	powershell -ExecutionPolicy Bypass -File .\\setup_localstack.ps1

deploy:
	powershell -ExecutionPolicy Bypass -File .\\deploy_all.ps1

migrate-up:
	go run ./cmd/runMigrations

run-subgraph:
	go run ./apps/subgraph-user
