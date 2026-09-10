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
- Expected tables: `TacoFoods`, `HouseholdMeasures` and `FoodSearchTokens`
- Expected GSI: `FoodNameIndex`

### `FoodSearchTokens` (word index for search)

The food search matches the term against the prefix of **any word** of the food
name, accent/case/punctuation insensitive (e.g. `requeij` finds
`Queijo, requeijão, cremoso`). It must not `Scan` `TacoFoods`, so there is a
dedicated index table, one row per distinct word of each food:

| Attribute        | Type   | Role                                                        |
| ---------------- | ------ | ---------------------------------------------------------- |
| `token_pl`       | S      | Partition key — first letter of the token (`_` if not a-z) |
| `tok`            | S      | Sort key — `<token>#<food_id>`                              |
| `token`          | S      | Normalized word                                            |
| `word_index`     | N      | Position of the word in the name (`0` = first word)         |
| `food_id`        | S      | Food id in `TacoFoods`                                      |
| `data_source`, `original_name`, `normalized_name`, `energy_kcal`, `protein_g`, `carbohydrate_g`, `fat_g`, `fiber_g` | — | Denormalized food fields so search resolves in a single `Query` |

Search runs `token_pl = :p AND begins_with(tok, :t)`. Partitioning by first
letter avoids a hot partition. Normalization/tokenization lives in
`internal/client/normalize.go` (`NormalizeTokens`) and is used both when writing
tokens and when searching, so they cannot diverge.

Create the table with partition key `token_pl` (S) and sort key `tok` (S),
on-demand billing, no GSI.

### Backfill

Every food write (`TacoRepository.PutFood`) also writes its token rows, so new
foods are searchable immediately. To index foods that already exist in
`TacoFoods`, run the backfill once after creating `FoodSearchTokens`:

```bash
go run ./cmd/backfill
```

It is idempotent (token keys are deterministic) — safe to re-run.

## Main Endpoints (base: /api)

- GET `/foods?search=rice`
- GET `/foods/{foodId}`
- GET `/foods/{foodId}/measures`
- POST `/plan/calculate`
