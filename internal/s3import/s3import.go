package s3import

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"

	svcerrors "project-serverless/internal/errors"
)

const (
	CSVContentType   = "text/csv"
	ReportContentType = "application/json"
)

// ObjectKeys returns deterministic S3 keys for CSV input and JSON report: {tenant}/{jobId}/input.csv and .../report.json.
func ObjectKeys(tenantID string, jobID uuid.UUID) (csvKey, reportKey string) {
	t := strings.ReplaceAll(strings.TrimSpace(tenantID), "/", "_")
	id := jobID.String()
	return fmt.Sprintf("%s/%s/input.csv", t, id),
		fmt.Sprintf("%s/%s/report.json", t, id)
}

// S3PresignAPI is the presign surface used by import flows (*s3.PresignClient implements it).
type S3PresignAPI interface {
	PresignPutObject(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.PresignOptions)) (*v4.PresignedHTTPRequest, error)
	PresignGetObject(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.PresignOptions)) (*v4.PresignedHTTPRequest, error)
}

// S3ObjectAPI is Head + Get + Put (*s3.Client implements it).
type S3ObjectAPI interface {
	S3HeadAPI
	GetObject(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error)
	PutObject(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error)
}

// PresignPut returns a presigned PUT URL and its TTL in seconds.
func PresignPut(ctx context.Context, client S3PresignAPI, bucket, key string, expiry time.Duration) (url string, expiresSec int, err error) {
	out, err := client.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(bucket),
		Key:         aws.String(key),
		ContentType: aws.String(CSVContentType),
	}, s3.WithPresignExpires(expiry))
	if err != nil {
		return "", 0, err
	}
	return out.URL, int(expiry.Seconds()), nil
}

// PresignGet returns a presigned GET URL and its TTL in seconds.
func PresignGet(ctx context.Context, client S3PresignAPI, bucket, key string, expiry time.Duration) (url string, expiresSec int, err error) {
	out, err := client.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	}, s3.WithPresignExpires(expiry))
	if err != nil {
		return "", 0, err
	}
	return out.URL, int(expiry.Seconds()), nil
}

// HeadObjectExists returns nil if object exists, or err if missing / other failure.
func HeadObjectExists(ctx context.Context, api S3HeadAPI, bucket, key string) error {
	_, err := api.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		if isNotFound(err) {
			return svcerrors.ImportS3Missing("object not found")
		}
		return svcerrors.ImportInternal("s3 head object failed", err)
	}
	return nil
}

// S3HeadAPI minimal surface for HeadObject (real *s3.Client implements this).
type S3HeadAPI interface {
	HeadObject(ctx context.Context, params *s3.HeadObjectInput, optFns ...func(*s3.Options)) (*s3.HeadObjectOutput, error)
}

func isNotFound(err error) bool {
	if err == nil {
		return false
	}
	// smithy API errors often include NotFound
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "notfound") || strings.Contains(s, "404") || strings.Contains(s, "nosuchkey")
}
