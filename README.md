# project-serverless

Go Lambdas (LocalStack / AWS), two GraphQL subgraphs (user + import), and workers for CSV import and CDC-driven read models.

## Architecture

Subgraphs run locally as HTTP servers (`ENVIRONMENT=local`). Each resolver invokes AWS Lambda by name unless you change the orchestrator.

### User Lambdas (subgraph-user → `http://localhost:4003/query`)

| Lambda (default name) | Source package | GraphQL surface |
|----------------------|----------------|-----------------|
| `createUser` | `cmd/createUser` | `mutation createUser` |
| `login` | `cmd/login` | `mutation login` |
| `updateUser` | `cmd/updateUser` | `mutation updateUser` |
| `deleteUser` | `cmd/deleteUser` | `mutation deleteUser` |
| `getUser` | `query/getUser` | `query getUser` |
| `listUsers` | `query/listUsers` | `query listUsers` |

Override names with env vars: `LAMBDA_CREATE_USER_NAME`, `LAMBDA_LOGIN_NAME`, `LAMBDA_UPDATE_USER_NAME`, `LAMBDA_DELETE_USER_NAME`, `LAMBDA_GET_USER_NAME`, `LAMBDA_LIST_USERS_NAME` (see `.env.example`).

### Import Lambdas (subgraph-import → `http://localhost:4004/query`)

| Lambda (default name) | Source package | GraphQL surface |
|----------------------|----------------|-----------------|
| `getImportUploadUrl` | `cmd/getImportUploadUrl` | `mutation getImportUploadUrl` |
| `startImport` | `cmd/startImport` | `mutation startImport` |
| `getImportJob` | `query/getImportJob` | `query getImportJob` |
| `listImportJobs` | `query/listImportJobs` | `query listImportJobs` |
| `getImportReportUrl` | `query/getImportReportUrl` | `query getImportReportUrl` |
| `importJobWorker` | `workers/importJobWorker` | SQS `import-jobs` (not GraphQL) |

Same pattern: `LAMBDA_GET_IMPORT_UPLOAD_URL_NAME`, `LAMBDA_START_IMPORT_NAME`, etc., in `.env.example`.

### Other workers

| Lambda | Role |
|--------|------|
| `cdcEventRouter` | Kinesis CDC stream → SQS |
| `userEventWorker` | SQS FIFO `users-events.fifo` → refresh read model |

## Local prerequisites

- Docker (LocalStack, Postgres/RDS as in your compose setup).
- Copy `.env.example` → `.env` and set `DATABASE_URL`, `JWT_SECRET`, `AWS_*` endpoints for Lambdas.
- From the repo root:
  - `make setup` — LocalStack, DB, migrations.
  - Optional: `make dms-setup` — DMS CDC (LocalStack Pro), before deploy if you use CDC.
  - `make deploy` — build and deploy all Lambdas to LocalStack.
- Run subgraphs (separate terminals):
  - `make run-subgraph` — user API on `:4003`
  - `make run-subgraph-import` — import API on `:4004`

---

## User GraphQL (examples)

Base URL: `http://localhost:4003/query`  
`Content-Type: application/json`

### Create user

```json
{
  "query": "mutation CreateUser($input: CreateUserInput!) { createUser(input: $input) { id tenantId username email createdAt } }",
  "variables": {
    "input": {
      "tenantId": "",
      "username": "Alice",
      "email": "alice@example.com",
      "password": "password123"
    }
  }
}
```

### Login (JWT for import + authenticated user ops)

```json
{
  "query": "mutation Login($email: String!, $password: String!) { login(email: $email, password: $password) { token } }",
  "variables": {
    "email": "alice@example.com",
    "password": "password123"
  }
}
```

Use header on later requests: `Authorization: Bearer <token>`.

### Get user

```json
{
  "query": "query GetUser($id: ID!) { getUser(id: $id) { id username email tenantId } }",
  "variables": { "id": "USER_ID_HERE" }
}
```

### List users

```json
{
  "query": "query ListUsers($skip: Int, $limit: Int) { listUsers(skip: $skip, limit: $limit) { items { id username email } total } }",
  "variables": { "skip": 0, "limit": 20 }
}
```

Optional filter:

```json
{
  "query": "query ListUsers($filter: UserListFilter) { listUsers(limit: 50, filter: $filter) { items { id email } total } }",
  "variables": { "filter": { "email": "alice" } }
}
```

### Update user

```json
{
  "query": "mutation UpdateUser($id: ID!, $input: UpdateUserInput!) { updateUser(id: $id, input: $input) { id username email } }",
  "variables": {
    "id": "USER_ID_HERE",
    "input": {
      "username": "Alice2",
      "email": "alice2@example.com"
    }
  }
}
```

### Delete user

```json
{
  "query": "mutation DeleteUser($id: ID!) { deleteUser(id: $id) { id } }",
  "variables": { "id": "USER_ID_HERE" }
}
```

---

## Import job flow (GraphQL + curl)

Import GraphQL: `http://localhost:4004/query`  
You need a user, JWT from **user** subgraph login, Lambdas deployed, S3 bucket and SQS queue configured (see deploy scripts / `.env.example`: `IMPORT_S3_BUCKET`, `IMPORT_QUEUE_URL`).

### 1) Login (user subgraph)

Use the **Login** mutation from the [User GraphQL](#user-graphql-examples) section against `http://localhost:4003/query`. Copy `token`.

### 2) Get upload URL + job id

Header: `Authorization: Bearer JWT_TOKEN_HERE`

```json
{
  "query": "mutation { getImportUploadUrl { url jobId csvS3Key expiresInSeconds } }"
}
```

```bash
curl --location 'http://localhost:4004/query' \
  --header 'Content-Type: application/json' \
  --header 'Authorization: Bearer JWT_TOKEN_HERE' \
  --data '{"query":"mutation { getImportUploadUrl { url jobId csvS3Key expiresInSeconds } }"}'
```

Keep `jobId` and presigned `url`.

### 3) Upload CSV to S3 (not GraphQL)

- `PUT` to the presigned `url`, body = raw CSV bytes.
- Do **not** send `Authorization` (signature is in the URL).
- `Content-Type: text/csv` (or what the presign expects).

Example:

```bash
curl --request PUT "PASTE_PRESIGNED_PUT_URL_HERE" \
  --header "Content-Type: text/csv" \
  --data-binary "@./users.csv"
```

CSV header must include:

```csv
full_name,email,age,phone_no,date_of_birth,gender
```

### 4) Start import

```json
{
  "query": "mutation StartImport($jobId: ID!) { startImport(jobId: $jobId) { jobId status } }",
  "variables": { "jobId": "PASTE_JOB_ID_HERE" }
}
```

```bash
curl --location 'http://localhost:4004/query' \
  --header 'Content-Type: application/json' \
  --header 'Authorization: Bearer JWT_TOKEN_HERE' \
  --data '{
    "query": "mutation StartImport($jobId: ID!) { startImport(jobId: $jobId) { jobId status } }",
    "variables": { "jobId": "PASTE_JOB_ID_HERE" }
  }'
```

This enqueues the job; `importJobWorker` processes it.

### 5) Job status

```json
{
  "query": "query GetImportJob($jobId: ID!) { getImportJob(jobId: $jobId) { id status totalRows passedRows failedRows errorMessage reportS3Key updatedAt } }",
  "variables": { "jobId": "PASTE_JOB_ID_HERE" }
}
```

Wait until status is `COMPLETED` (or `FAILED`).

### 6) Report download URL

```json
{
  "query": "query GetImportReportUrl($jobId: ID!) { getImportReportUrl(jobId: $jobId) { url expiresInSeconds } }",
  "variables": { "jobId": "PASTE_JOB_ID_HERE" }
}
```

```bash
curl --location 'http://localhost:4004/query' \
  --header 'Content-Type: application/json' \
  --header 'Authorization: Bearer JWT_TOKEN_HERE' \
  --data '{
    "query": "query GetImportReportUrl($jobId: ID!) { getImportReportUrl(jobId: $jobId) { url expiresInSeconds } }",
    "variables": { "jobId": "PASTE_JOB_ID_HERE" }
  }'
```

### 7) Download report JSON (not GraphQL)

`GET` the presigned `url` with **no** bearer token:

```bash
curl --location "PASTE_PRESIGNED_REPORT_URL_HERE" --output report.json
```

---

## Makefile quick reference

| Target | Purpose |
|--------|---------|
| `make test` | `go test ./...` |
| `make setup` | LocalStack + DB + migrations |
| `make dms-setup` | DMS CDC (after setup, optional) |
| `make deploy` | Build and deploy Lambdas |
| `make run-subgraph` | User GraphQL `:4003` |
| `make run-subgraph-import` | Import GraphQL `:4004` |

---

## Common mistakes

- **Presigned S3 URLs**: do not add `Authorization: Bearer ...` on PUT/GET to S3.
- **Upload body**: send file **bytes**, not a path string as JSON.
- **Expired URLs**: presign TTL is often 900 seconds; repeat mutation if needed.
- **Wrong `jobId`**: must match the job from `getImportUploadUrl`.
- **CSV header**: must match validators (`full_name`, `email`, etc.).
- **Import subgraph without Lambdas**: resolvers invoke Lambda; LocalStack must have functions deployed and `.env` must point `AWS_LAMBDA_ENDPOINT` / `AWS_ENDPOINT_URL` correctly.
