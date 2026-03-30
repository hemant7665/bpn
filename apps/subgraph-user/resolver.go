package main

import "project-serverless/apps/subgraph-user/orchestrator"

// This file will not be regenerated automatically.
//
// It serves as dependency injection for your app, add any dependencies you require
// here.

type Resolver struct {
	Orchestrator orchestrator.Service
}
