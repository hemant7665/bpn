.PHONY: help test setup deploy dms-setup run-subgraph run-subgraph-import test-import-lambdas

help:
	@echo "Available targets:"
	@echo "  test           - Run all Go tests"
	@echo "  setup          - LocalStack + RDS + migrations"
	@echo "  dms-setup      - LocalStack Pro DMS CDC (run after setup, before deploy)"
	@echo "  deploy         - Deploy Lambdas to LocalStack"
	@echo "  test-import-lambdas - After setup+deploy: invoke createUser, login, getImportUploadUrl, list/get import jobs"
	@echo "  run-subgraph        - Run user GraphQL subgraph locally (:4003)"
	@echo "  run-subgraph-import - Run import GraphQL subgraph locally (:4004)"
	@echo ""
	@echo "Lambdas: user + import (getImportUploadUrl,startImport,getImportJob,listImportJobs,getImportReportUrl,importJobWorker) + CDC workers"



test:
	go test ./...



run-subgraph:
	go run ./apps/subgraph-user

run-subgraph-import:
	go run ./apps/subgraph-import
