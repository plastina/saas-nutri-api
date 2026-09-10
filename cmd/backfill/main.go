// Command backfill (re)gera as linhas da tabela FoodSearchTokens para todos os
// alimentos ja existentes na TacoFoods.
//
// E idempotente: as chaves dos tokens sao deterministicas, entao rodar de novo
// apenas sobrescreve linhas iguais. Rode uma vez apos criar a tabela
// FoodSearchTokens; alimentos gravados pela API depois disso ja saem indexados.
//
//	go run ./cmd/backfill
package main

import (
	"context"
	"log"

	"saas-nutri/internal/client"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/joho/godotenv"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("Aviso: .env nao encontrado; usando credenciais do ambiente.")
	}

	ctx := context.Background()
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion("sa-east-1"))
	if err != nil {
		log.Fatalf("erro ao carregar configuracao AWS: %v", err)
	}
	db := dynamodb.NewFromConfig(cfg)

	const tacoTable = "TacoFoods"
	repo := client.NewTacoRepository(db, tacoTable, "FoodSearchTokens")

	var (
		startKey map[string]types.AttributeValue
		total    int
	)
	for {
		out, err := db.Scan(ctx, &dynamodb.ScanInput{
			TableName:         aws.String(tacoTable),
			ExclusiveStartKey: startKey,
		})
		if err != nil {
			log.Fatalf("erro ao percorrer %s: %v", tacoTable, err)
		}

		var foods []client.TacoFoodItem
		if err := attributevalue.UnmarshalListOfMaps(out.Items, &foods); err != nil {
			log.Fatalf("erro ao ler alimentos: %v", err)
		}

		for _, f := range foods {
			if err := repo.PutFoodTokens(ctx, f); err != nil {
				log.Fatalf("erro ao gravar tokens do alimento %s: %v", f.FoodID, err)
			}
			total++
		}
		log.Printf("processados %d alimentos...", total)

		if len(out.LastEvaluatedKey) == 0 {
			break
		}
		startKey = out.LastEvaluatedKey
	}

	log.Printf("backfill concluido: %d alimentos indexados na FoodSearchTokens", total)
}
