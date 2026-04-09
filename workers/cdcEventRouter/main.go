// cdcEventRouter consumes the CDC Kinesis stream (DMS target) and routes write_model.users events to SQS FIFO.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	lambdaevents "github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sqs"

	svcerrors "project-serverless/internal/errors"
	"project-serverless/internal/logger"
)

// CDCEvent is the envelope sent to SQS (same shape as DMS Kinesis JSON; aligns with BluePrint cdcEventRouter).
type CDCEvent struct {
	Metadata struct {
		Operation string `json:"operation"`
		TableName string `json:"table-name"`
		Schema    string `json:"schema-name"`
	} `json:"metadata"`
	Data map[string]interface{} `json:"data"`
}

var (
	sqsClient     *sqs.Client
	usersQueueURL string
)

func initAWS(ctx context.Context) error {
	if sqsClient != nil {
		return nil
	}
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return svcerrors.Internal("failed to load AWS config", err)
	}
	if os.Getenv("ENVIRONMENT") == "local" && os.Getenv("AWS_ENDPOINT_URL") != "" {
		cfg.BaseEndpoint = aws.String(os.Getenv("AWS_ENDPOINT_URL"))
	}
	sqsClient = sqs.NewFromConfig(cfg)

	q := os.Getenv("USERS_EVENTS_QUEUE_NAME")
	if q == "" {
		q = "users-events.fifo"
	}
	out, err := sqsClient.GetQueueUrl(ctx, &sqs.GetQueueUrlInput{QueueName: aws.String(q)})
	if err != nil {
		return svcerrors.Internal(fmt.Sprintf("get queue url %s", q), err)
	}
	usersQueueURL = *out.QueueUrl
	return nil
}

func HandleRequest(ctx context.Context, kinesisEvent lambdaevents.KinesisEvent) error {
	if err := initAWS(ctx); err != nil {
		return err
	}
	for _, record := range kinesisEvent.Records {
		cev, skip, skipReason, err := parseKinesisJSON(record.Kinesis.Data)
		if err != nil {
			logger.Error("cdc_router_unmarshal", map[string]any{"error": err.Error()})
			continue
		}
		if skip {
			if !strings.HasPrefix(skipReason, "record_type:") {
				logger.Info("cdc_router_skip", map[string]any{"reason": skipReason})
			}
			continue
		}
		if !strings.EqualFold(strings.TrimSpace(cev.Metadata.TableName), "users") {
			logger.Info("cdc_router_skip_table", map[string]any{"table": cev.Metadata.TableName})
			continue
		}
		if !schemaAllowsWriteModelUsers(cev.Metadata.Schema) {
			logger.Info("cdc_router_skip_schema", map[string]any{"schema": cev.Metadata.Schema})
			continue
		}
		body, err := json.Marshal(cev)
		if err != nil {
			return err
		}
		entityID := fmt.Sprint(cev.Data["id"])
		if entityID == "" || entityID == "<nil>" {
			entityID = record.Kinesis.SequenceNumber
		}
		_, err = sqsClient.SendMessage(ctx, &sqs.SendMessageInput{
			QueueUrl:               aws.String(usersQueueURL),
			MessageBody:            aws.String(string(body)),
			MessageGroupId:         aws.String(entityID),
			MessageDeduplicationId: aws.String(record.Kinesis.SequenceNumber + "-users"),
		})
		if err != nil {
			return err
		}
		logger.Info("cdc_router_routed", map[string]any{"table": "users", "op": cev.Metadata.Operation})
	}
	return nil
}

// schemaAllowsWriteModelUsers returns whether CDC metadata schema should be accepted for write_model.users routing.
// LocalStack DMS may emit "public" for PostgreSQL; allow that only when ENVIRONMENT=local.
func schemaAllowsWriteModelUsers(schema string) bool {
	s := strings.TrimSpace(schema)
	if s == "" || strings.EqualFold(s, "write_model") {
		return true
	}
	if os.Getenv("ENVIRONMENT") == "local" && strings.EqualFold(s, "public") {
		return true
	}
	return false
}

func stringFromMap(m map[string]interface{}, keys ...string) string {
	for _, k := range keys {
		v, ok := m[k]
		if !ok || v == nil {
			continue
		}
		switch t := v.(type) {
		case string:
			if s := strings.TrimSpace(t); s != "" {
				return s
			}
		case float64:
			return fmt.Sprint(t)
		case json.Number:
			return strings.TrimSpace(t.String())
		}
	}
	return ""
}

// parseKinesisJSON unmarshals one Kinesis record payload from DMS (JSON / JSON_UNFORMATTED single object).
// Non-data control/DDL records (when record-type is present and not "data") are skipped.
func parseKinesisJSON(raw []byte) (ev CDCEvent, skip bool, skipReason string, err error) {
	var root map[string]interface{}
	if err := json.Unmarshal(raw, &root); err != nil {
		return CDCEvent{}, false, "", err
	}
	meta, ok := root["metadata"].(map[string]interface{})
	if !ok || meta == nil {
		return CDCEvent{}, true, "no_metadata", nil
	}
	rt := strings.ToLower(strings.TrimSpace(stringFromMap(meta, "record-type", "record_type", "recordType")))
	if rt != "" && rt != "data" {
		return CDCEvent{}, true, "record_type:" + rt, nil
	}
	op := stringFromMap(meta, "operation", "Operation")
	table := stringFromMap(meta, "table-name", "table_name", "tableName")
	schema := stringFromMap(meta, "schema-name", "schema_name", "schemaName")
	data, _ := root["data"].(map[string]interface{})
	if data == nil {
		data = map[string]interface{}{}
	}
	ev.Metadata.Operation = op
	ev.Metadata.TableName = table
	ev.Metadata.Schema = schema
	ev.Data = data
	return ev, false, "", nil
}

func main() {
	lambda.Start(HandleRequest)
}
