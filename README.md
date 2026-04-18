# Saas Nutri API

Go API for the SaaS Nutri nutrition domain.

## What This Project Does

- Searches foods by name.
- Returns food details and household measures.
- Calculates calories per item and totals per meal.
- Exposes Swagger documentation for integration.

## Client Context

This API serves the Angular frontend in the monorepo:

1. The client sends search terms and plan payloads.
2. The API queries nutrition data in DynamoDB.
3. The API returns foods, measures, and calculation results.
4. The client renders results for meal-plan editing and review.

## Requirements

- Go 1.24
- Valid AWS credentials for DynamoDB

Security note:

- Do not commit credentials, tokens, secrets, or personal data.

## Run Locally

```bash
go mod download
go run ./cmd/server
```

API: `http://localhost:8080`
Swagger: `http://localhost:8080/swagger/index.html`

## Data Dependencies (DynamoDB)

- Region configured in code: `sa-east-1`
- Expected tables: `TacoFoods` and `HouseholdMeasures`
- Expected GSI: `FoodNameIndex`

## Main Endpoints (base: /api)

- GET `/foods?search=rice`
- GET `/foods/{foodId}`
- GET `/foods/{foodId}/measures`
- POST `/plan/calculate`
