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
  - `TacoFoods` holds both TACO items (`data_source = "TACO"`) and, after running
    `cmd/importoff`, Open Food Facts items (`data_source = "OFF"`, id `off-<code>`).
- GSI `FoodNameIndex` (on `TacoFoods`): **no longer used by the code.** Search
  moved to the `FoodSearchTokens` table; this GSI is now dead weight and can be
  deleted manually in the AWS console. Left in place here since removing it is an
  infra change outside the codebase.

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

## Open Food Facts import (`cmd/importoff`)

The TACO base has no processed/industrialized products or supplements (e.g. whey
protein). `cmd/importoff` is a one-shot loader that adds Brazilian products from
**Open Food Facts (OFF)** to `TacoFoods` with `data_source = "OFF"`.

### Where to get the dump

The command reads a **local file already downloaded** — it does **not** call the
OFF API (which is rate-limited). Download one of the full exports from
<https://world.openfoodfacts.org/data> and decompress it:

- **CSV** (`en.openfoodfacts.org.products.csv.gz`) — **tab-separated** despite the
  `.csv` name; this is why `-delimiter` defaults to a tab.
- **JSONL** (`openfoodfacts-products.jsonl.gz`) — one product JSON per line.

Only these fields are used: `code`, `product_name`, `brands`, `countries_tags`,
and the nutriments `energy-kcal_100g`, `proteins_100g`, `carbohydrates_100g`,
`fat_100g`. Every other nutrient is discarded.

### Run

```bash
# dry-run: parse + print the summary, write nothing
go run ./cmd/importoff -file ./data/en.openfoodfacts.org.products.csv -dry-run

# real load
go run ./cmd/importoff -file ./data/en.openfoodfacts.org.products.csv
go run ./cmd/importoff -file ./data/openfoodfacts-products.jsonl
```

Flags: `-format csv|jsonl` (default: by file extension), `-delimiter` (CSV only,
default tab), `-batch` (items per batch write, default 500), `-dry-run`.

### Behaviour

- Imports **only products whose country is Brazil**.
- Food name = product name + first brand, e.g.
  `Whey Protein Concentrado (Growth Supplements)`. Search tokenization treats the
  parentheses as separators, so the item is found by both the product and the
  brand words.
- Maps to the current model: name, kcal, protein, carbohydrate, fat **per 100 g**.
- A product is **discarded** when: empty product name; missing OFF code; kcal or
  any of the three macros absent; kcal or a macro negative; macro sum above
  100 g per 100 g; kcal outside 0–900 per 100 g; or declared kcal diverging from
  the macro estimate (4/4/9) beyond the tolerance in
  `internal/client/off_import.go`.
- **Idempotent**: the id is `off-<code>`, so re-running overwrites the same item
  without creating duplicates. TACO items (numeric ids, referenced by saved
  plans) are never read or modified, and `HouseholdMeasures` is untouched — OFF
  foods have grams only.
- At the end it prints a summary: records read, imported, discarded, and the
  count per discard reason.

DynamoDB access stays in `internal/client` (`TacoRepository.PutFoodsBatch`),
which batch-writes with the same throttling handling as the token backfill.

### Attribution (ODbL) — required

Open Food Facts data is published under the **Open Database License (ODbL) v1.0**
and individual contents under the **Database Contents License (DbCL) v1.0**. Any
product/UI that surfaces data loaded by this command **must** credit the source
and license, e.g.:

> Nutrition data for processed products from **Open Food Facts**
> (<https://world.openfoodfacts.org>), used under the Open Database License
> (ODbL) v1.0 — <https://opendatacommons.org/licenses/odbl/1-0/>.

ODbL is share-alike: a redistributed database derived from OFF data must be
offered under the ODbL as well.

## Main Endpoints (base: /api)

- GET `/foods?search=rice`
- GET `/foods/{foodId}`
- GET `/foods/{foodId}/measures`
- POST `/plan/calculate`
