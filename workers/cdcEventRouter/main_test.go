package main

import (
	"testing"
)

func TestParseKinesisJSON_awsShape(t *testing.T) {
	raw := []byte(`{
		"data": {"id": "x", "email": "a@b.c"},
		"metadata": {
			"record-type": "data",
			"operation": "insert",
			"schema-name": "write_model",
			"table-name": "users"
		}
	}`)
	ev, skip, reason, err := parseKinesisJSON(raw)
	if err != nil || skip {
		t.Fatalf("err=%v skip=%v reason=%s", err, skip, reason)
	}
	if ev.Metadata.TableName != "users" || ev.Metadata.Schema != "write_model" || ev.Metadata.Operation != "insert" {
		t.Fatalf("metadata: %+v", ev.Metadata)
	}
	if ev.Data["id"] != "x" {
		t.Fatalf("data: %v", ev.Data)
	}
}

func TestParseKinesisJSON_snakeCaseMetadata(t *testing.T) {
	raw := []byte(`{"data":{"id":"1"},"metadata":{"operation":"update","schema_name":"write_model","table_name":"users"}}`)
	ev, skip, _, err := parseKinesisJSON(raw)
	if err != nil || skip {
		t.Fatal(err, skip)
	}
	if ev.Metadata.TableName != "users" || ev.Metadata.Schema != "write_model" {
		t.Fatalf("%+v", ev.Metadata)
	}
}

func TestParseKinesisJSON_skipsControl(t *testing.T) {
	raw := []byte(`{"metadata":{"record-type":"control"}}`)
	_, skip, reason, err := parseKinesisJSON(raw)
	if err != nil || !skip || reason == "" {
		t.Fatalf("err=%v skip=%v reason=%q", err, skip, reason)
	}
}

func TestSchemaAllows_publicWhenLocal(t *testing.T) {
	t.Setenv("ENVIRONMENT", "local")
	if !schemaAllowsWriteModelUsers("public") {
		t.Fatal("expected public allowed in local")
	}
}

func TestSchemaAllows_rejectPublicWhenNotLocal(t *testing.T) {
	t.Setenv("ENVIRONMENT", "staging")
	if schemaAllowsWriteModelUsers("public") {
		t.Fatal("public must not be allowed outside local")
	}
}

func TestSchemaAllows_emptyAndWriteModel(t *testing.T) {
	t.Setenv("ENVIRONMENT", "")
	if !schemaAllowsWriteModelUsers("") || !schemaAllowsWriteModelUsers("write_model") {
		t.Fatal()
	}
}
