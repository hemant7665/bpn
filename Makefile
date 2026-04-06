.PHONY: help test setup deploy dms-setup run-subgraph

help:
	@echo "Available targets:"
	@echo "  test           - Run all Go tests"
	@echo "  setup          - LocalStack + RDS + migrations"
	@echo "  dms-setup      - LocalStack Pro DMS CDC (run after setup, before deploy)"
	@echo "  deploy         - Deploy Lambdas to LocalStack"
	@echo "  run-subgraph   - Run GraphQL subgraph locally"
	@echo ""
	@echo "Lambdas: cmd/createUser,updateUser,deleteUser | query/getUser,listUsers,login | workers/cdcEventRouter,userEventWorker"

test:
	go test ./...

setup:
	powershell -ExecutionPolicy Bypass -File .\\setup_localstack.ps1

deploy:
	powershell -ExecutionPolicy Bypass -File .\\deploy_all.ps1

dms-setup:
	powershell -ExecutionPolicy Bypass -File .\\setup_dms_cdc.ps1

run-subgraph:
	go run ./apps/subgraph-user
