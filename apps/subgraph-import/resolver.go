package main

import "project-serverless/apps/subgraph-import/orchestrator"

// Resolver wires GraphQL to the import orchestrator.
type Resolver struct {
	Orchestrator orchestrator.Service
}
