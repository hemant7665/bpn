# setup_localstack.ps1
# Bootstrap LocalStack RDS for project-serverless.
# Uses 'awslocal' inside the container - no host AWS CLI needed.
# Run from the project-serverless directory: .\setup_localstack.ps1

$DBID   = "project-db"
$DBUSR  = "postgres"
$DBPWD  = "POSTGRES"
$DBPORT = "4510"
$DBNAME = "lamdapractice"
$CTR    = "localstack_project"
$MFILE  = "internal\scripts\migrations\001_user.sql"

function Step([string]$t) {
    Write-Host ""
    Write-Host "===> $t" -ForegroundColor Cyan
}

# ── 1. Start LocalStack ────────────────────────────────────────
Step "1. Starting LocalStack (docker-compose up -d)..."
docker-compose up -d
if ($LASTEXITCODE -ne 0) { Write-Host "ERROR: docker-compose failed" -ForegroundColor Red; exit 1 }
Write-Host "Waiting 15s for LocalStack to start..." -ForegroundColor Yellow
Start-Sleep -Seconds 15

# ── 2. Create RDS instance (using awslocal inside container) ───
Step "2. Creating LocalStack RDS instance ($DBID)..."
$r = docker exec $CTR awslocal rds create-db-instance --db-instance-identifier $DBID --db-instance-class db.t3.micro --engine postgres --master-username $DBUSR --master-user-password $DBPWD --allocated-storage 20 2>&1
if ($LASTEXITCODE -eq 0) { Write-Host "RDS instance created." -ForegroundColor Green }
else { Write-Host "RDS may already exist - continuing." -ForegroundColor Yellow }

# ── 3. Wait for RDS to become available ───────────────────────
Step "3. Waiting for RDS to become available..."
$maxWait = 12
$ready = $false
for ($i = 1; $i -le $maxWait; $i++) {
    Write-Host "  Attempt ${i}/${maxWait}..." -NoNewline
    $status = docker exec $CTR awslocal rds describe-db-instances --db-instance-identifier $DBID --query "DBInstances[0].DBInstanceStatus" --output text 2>&1
    Write-Host " Status: $status"
    if ($status -eq "available") { $ready = $true; break }
    Start-Sleep -Seconds 5
}
if (-not $ready) { Write-Host "WARNING: RDS not yet 'available', continuing anyway..." -ForegroundColor Yellow }

# ── 4. Install psql in container ───────────────────────────────
Step "4. Installing postgresql-client..."
docker exec $CTR bash -c "apt-get update -qq && apt-get install -y -qq postgresql-client > /dev/null 2>&1 && echo psql_ready"

# ── 5. Create the database ─────────────────────────────────────
Step "5. Creating database '$DBNAME'..."
$SYSDSN = "postgres://${DBUSR}:${DBPWD}@localhost:${DBPORT}/postgres?sslmode=disable"
docker exec $CTR bash -c "psql '$SYSDSN' -c 'CREATE DATABASE $DBNAME;' || true"
Write-Host "Waiting 3s for database catalog to update..." -ForegroundColor Yellow
Start-Sleep -Seconds 3
Write-Host "Database step done." -ForegroundColor Green

# ── 6. Run migrations ──────────────────────────────────────────
Step "6. Running migration ($MFILE)..."
if (-not (Test-Path $MFILE)) {
    Write-Host "ERROR: Migration file not found: $MFILE" -ForegroundColor Red
    exit 1
}
docker cp $MFILE "${CTR}:/tmp/migration.sql"
if ($LASTEXITCODE -ne 0) { Write-Host "ERROR: docker cp failed" -ForegroundColor Red; exit 1 }
$DBDSN = "postgres://${DBUSR}:${DBPWD}@localhost:${DBPORT}/${DBNAME}?sslmode=disable"
docker exec $CTR bash -c "psql '$DBDSN' -f /tmp/migration.sql"
if ($LASTEXITCODE -ne 0) { Write-Host "ERROR: Migration failed" -ForegroundColor Red; exit 1 }
Write-Host "Migration applied." -ForegroundColor Green

# ── 7. Verify ─────────────────────────────────────────────────
Step "7. Verifying tables..."
docker exec $CTR bash -c "psql '$DBDSN' -c '\dt write_model.*'"
docker exec $CTR bash -c "psql '$DBDSN' -c '\dm read_model.*'"

Write-Host ""
Write-Host "=============================================" -ForegroundColor Green
Write-Host "  Setup complete!" -ForegroundColor Green
Write-Host "  DB  : localhost.localstack.cloud:${DBPORT}/${DBNAME}" -ForegroundColor Yellow
Write-Host "  Next: .\deploy_all.ps1" -ForegroundColor Yellow
Write-Host "=============================================" -ForegroundColor Green
