# ============================================================
# deploy_all.ps1
# Builds all Lambdas for Linux/arm64 and deploys them to
# LocalStack using the correct LocalStack RDS DATABASE_URL.
#
# Run from the project-serverless directory:
#   .\deploy_all.ps1
# ============================================================

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

# ── Configuration ─────────────────────────────────────────────
$LOCALSTACK_URL  = "http://localhost:4566"
$CONTAINER_NAME  = "localstack_project"
$S3_BUCKET       = "lambda-code"
$IAM_ROLE        = "arn:aws:iam::000000000000:role/lambda-role"

# LocalStack RDS endpoint — resolvable from within Lambda containers.
$DATABASE_URL    = "postgres://postgres:POSTGRES@localhost.localstack.cloud:4510/lamdapractice?sslmode=disable"

# AWS_ENDPOINT_URL passed INTO the Lambda env so the Lambda can reach
# LocalStack services (SQS, Kinesis, etc.) container-to-container.
$LAMBDA_AWS_ENDPOINT = "http://${CONTAINER_NAME}:4566"

$functions = @(
    @{ name = "createUser";           path = "./cmd/createUser"  },
    @{ name = "updateUser";           path = "./cmd/updateUser"  },
    @{ name = "deleteUser";           path = "./cmd/deleteUser"  },
    @{ name = "getUser";              path = "./query/getUser"   },
    @{ name = "userSyncWorker";       path = "./worker/user"     }
)

# ── Helpers ───────────────────────────────────────────────────
function Invoke-Awslocal {
    param([string[]]$Args)
    docker exec $CONTAINER_NAME awslocal @Args
}

function Write-Step([string]$msg) {
    Write-Host ""
    Write-Host "===> $msg" -ForegroundColor Cyan
}

# ── Pre-flight: ensure S3 bucket and IAM role exist ──────────
Write-Step "Ensuring S3 bucket '$S3_BUCKET' exists..."
$buckets = docker exec $CONTAINER_NAME awslocal s3 ls
if ($buckets -notmatch $S3_BUCKET) {
    docker exec $CONTAINER_NAME awslocal s3 mb "s3://$S3_BUCKET"
    Write-Host "Bucket created." -ForegroundColor Green
} else {
    Write-Host "Bucket already exists." -ForegroundColor Yellow
}

Write-Step "Ensuring IAM role exists..."
$roles = docker exec $CONTAINER_NAME awslocal iam list-roles --query "Roles[].RoleName" --output text
if ($roles -notmatch "lambda-role") {
    docker exec $CONTAINER_NAME awslocal iam create-role `
        --role-name lambda-role `
        --assume-role-policy-document '{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":{"Service":"lambda.amazonaws.com"},"Action":"sts:AssumeRole"}]}' | Out-Null
    Write-Host "IAM role created." -ForegroundColor Green
} else {
    Write-Host "IAM role already exists." -ForegroundColor Yellow
}

# ── SQS Configuration ──────────────────────────────────────────
Write-Host "===> Ensuring SQS queues exist..."

$QUEUES = @("user-create-queue", "user-update-queue", "user-delete-queue")
foreach ($q in $QUEUES) {
    docker exec $CONTAINER_NAME awslocal sqs create-queue --queue-name $q | Out-Null
    Write-Host "  Queue ready: $q"
}

# Base URL for internal Lambda-to-SQS communication
$SQS_BASE_URL = "http://sqs.us-east-1.localhost.localstack.cloud:4566/000000000000"
$SQS_QUEUE_URL  = "$SQS_BASE_URL/user-create-queue"  # Used by createUser

# ── Deploy each function ──────────────────────────────────────
Write-Host ""
Write-Host "=== FLEET DEPLOYMENT STARTING ===" -ForegroundColor Cyan

foreach ($f in $functions) {
    Write-Step "[$($f.name)] Building from $($f.path)..."

    $binaryName = "$($f.name)_bin"

    # Build inside a golang:1.23 container — produces a real Linux arm64 binary
    docker run --rm `
        -v "$(Get-Location):/app" `
        -w /app `
        -e GOOS=linux `
        -e GOARCH=arm64 `
        -e CGO_ENABLED=0 `
        golang:1.25 `
        go build -tags lambda.norpc -ldflags="-s -w" -o "./volume/$binaryName" $($f.path)

    if ($LASTEXITCODE -ne 0) {
        Write-Host "ERROR: Build failed for $($f.name). Skipping." -ForegroundColor Red
        continue
    }
    Write-Host "  Build OK." -ForegroundColor Green

    # B. Transfer & Zip inside LocalStack (correct permissions)
    Write-Host "  Transferring and zipping..."
    
    # Copy from host's ./volume/ to container's /tmp/bootstrap
    docker cp "./volume/$binaryName" "${CONTAINER_NAME}:/tmp/bootstrap"
    
    docker exec $CONTAINER_NAME bash -c "chmod +x /tmp/bootstrap && cd /tmp && zip -j -q /tmp/$($f.name).zip bootstrap"
    
    # C. Upload to S3
    Write-Host "  Uploading to S3..."
    docker exec $CONTAINER_NAME awslocal s3 cp "/tmp/$($f.name).zip" "s3://$S3_BUCKET/$($f.name).zip" | Out-Null
    
    # D. Lambda environment variables
    $envVars = "Variables={DATABASE_URL=$DATABASE_URL,SQS_QUEUE_URL=$SQS_QUEUE_URL,AWS_ENDPOINT_URL=$LAMBDA_AWS_ENDPOINT,AWS_REGION=us-east-1,AWS_ACCESS_KEY_ID=test,AWS_SECRET_ACCESS_KEY=test}"

    # E. Create or update the Lambda function
    # Use a robust bash-level check to see if the function exists
    $exists = docker exec $CONTAINER_NAME bash -c "awslocal lambda get-function --function-name $($f.name) >/dev/null 2>&1 && echo yes || echo no"
    
    if ($exists -notmatch "yes") {
        Write-Host "  Creating Lambda '$($f.name)'..."
        docker exec $CONTAINER_NAME awslocal lambda create-function `
            --function-name $($f.name) `
            --runtime provided.al2 `
            --role $IAM_ROLE `
            --handler bootstrap `
            --code "S3Bucket=$S3_BUCKET,S3Key=$($f.name).zip" `
            --timeout 30 `
            --memory-size 256 `
            --environment $envVars | Out-Null
    } else {
        Write-Host "  Updating Lambda '$($f.name)'..."
        docker exec $CONTAINER_NAME awslocal lambda update-function-code `
            --function-name $($f.name) `
            --s3-bucket $S3_BUCKET `
            --s3-key "$($f.name).zip" | Out-Null

        # Wait for update to complete before changing config
        Start-Sleep -Seconds 1

        docker exec $CONTAINER_NAME awslocal lambda update-function-configuration `
            --function-name $($f.name) `
            --timeout 30 `
            --environment $envVars | Out-Null
    }

    # F. Add Event Source Mappings for all 3 queues → userSyncWorker
    if ($f.name -eq "userSyncWorker") {
        Write-Host "  Mapping SQS triggers for userSyncWorker..."
        $mappings = docker exec $CONTAINER_NAME awslocal lambda list-event-source-mappings `
            --function-name userSyncWorker --output text 2>&1

        foreach ($q in $QUEUES) {
            $qARN = "arn:aws:sqs:us-east-1:000000000000:$q"
            if ($mappings -match $q) {
                Write-Host "    $q already mapped."
            } else {
                docker exec $CONTAINER_NAME awslocal lambda create-event-source-mapping `
                    --function-name userSyncWorker `
                    --event-source-arn $qARN `
                    --batch-size 10 | Out-Null
                Write-Host "    $q mapped!" -ForegroundColor Green
            }
        }
    }

    Write-Host "  SUCCESS: $($f.name) is live!" -ForegroundColor Green
}

# ── Summary ───────────────────────────────────────────────────
Write-Host ""
Write-Host "=== FLEET DEPLOYMENT COMPLETE ===" -ForegroundColor Cyan
Write-Host ""
Write-Host "Deployed functions:" -ForegroundColor White
docker exec $CONTAINER_NAME awslocal lambda list-functions `
    --query "Functions[].FunctionName" `
    --output table

Write-Host ""
Write-Host "DATABASE_URL injected into Lambdas:" -ForegroundColor White
Write-Host "  $DATABASE_URL" -ForegroundColor Yellow
Write-Host ""
Write-Host "Test createUser with:" -ForegroundColor White
Write-Host @"
  aws --endpoint-url=http://localhost:4566 lambda invoke `
    --function-name createUser `
    --payload '{"name":"Alice","email":"alice@example.com"}' `
    --cli-binary-format raw-in-base64-out `
    response.json && cat response.json
"@ -ForegroundColor Yellow
