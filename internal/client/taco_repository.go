package client

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

type TacoFoodItem struct {
	FoodID          string  `dynamodbav:"food_id"`
	DataSource      string  `dynamodbav:"data_source"`
	NormalizedName  string  `dynamodbav:"normalized_name"`
	OriginalName    string  `dynamodbav:"original_name"`
	EnergyKcal      float64 `dynamodbav:"energy_kcal,omitempty"`
	ProteinG        float64 `dynamodbav:"protein_g,omitempty"`
	CarbohydrateG   float64 `dynamodbav:"carbohydrate_g,omitempty"`
	FatG            float64 `dynamodbav:"fat_g,omitempty"`
	FiberG          float64 `dynamodbav:"fiber_g,omitempty"`
}

type TacoRepository struct {
	DB        *dynamodb.Client
	TableName string
	IndexName string
}

type MeasureItem struct {
	MeasureName    string  `json:"measure_name" dynamodbav:"measure_name"`
	DisplayName    string  `json:"display_name" dynamodbav:"display_name"`
	GramEquivalent float64 `json:"gram_equivalent" dynamodbav:"gram_equivalent"`
}

func NewTacoRepository(db *dynamodb.Client, tableName, indexName string) *TacoRepository {
	return &TacoRepository{DB: db, TableName: tableName, IndexName: indexName}
}

func normalizeString(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

func (r *TacoRepository) SearchFoodsByNamePrefix(ctx context.Context, namePrefix string) ([]TacoFoodItem, error) {
	var items []TacoFoodItem
	normalizedPrefix := normalizeString(namePrefix)

	if normalizedPrefix == "" {
		return items, nil
	}

	keyConditionExpression := "data_source = :ds AND begins_with(normalized_name, :prefix)"
	expressionAttributeValues := map[string]types.AttributeValue{
		":ds":     &types.AttributeValueMemberS{Value: "TACO"},
		":prefix": &types.AttributeValueMemberS{Value: normalizedPrefix},
	}

	projectionExpression := "food_id, data_source, normalized_name, original_name, energy_kcal, protein_g, carbohydrate_g, fat_g, fiber_g"

	queryInput := &dynamodb.QueryInput{
		TableName:                 aws.String(r.TableName), 
		IndexName:                 aws.String(r.IndexName), 
		KeyConditionExpression:    aws.String(keyConditionExpression),
		ExpressionAttributeValues: expressionAttributeValues,
		ProjectionExpression:      aws.String(projectionExpression),
		Limit:                     aws.Int32(25),
	}

	log.Printf("Executando Query no DynamoDB GSI '%s' com prefixo: '%s'", r.IndexName, normalizedPrefix)

	result, err := r.DB.Query(ctx, queryInput)
	if err != nil {
		return nil, fmt.Errorf("erro ao executar query no DynamoDB: %w", err)
	}

	err = attributevalue.UnmarshalListOfMaps(result.Items, &items)
	if err != nil {
		return nil, fmt.Errorf("erro ao fazer unmarshal dos resultados do DynamoDB: %w", err)
	}

	log.Printf("DynamoDB Query retornou %d itens", len(items))

	return items, nil
}

func (r *TacoRepository) GetMeasuresForFood(ctx context.Context, foodID string) ([]MeasureItem, error) {
	var items []MeasureItem

	defaultMeasure := MeasureItem{
		MeasureName:    "grama",
		DisplayName:    "Grama",
		GramEquivalent: 1.0,
	}
	items = append(items, defaultMeasure)

	keyConditionExpression := "food_id = :fid"
	expressionAttributeValues := map[string]types.AttributeValue{
		":fid": &types.AttributeValueMemberS{Value: foodID},
	}

	projectionExpression := "measure_name, display_name, gram_equivalent"

	queryInput := &dynamodb.QueryInput{
		TableName:                 aws.String("HouseholdMeasures"), 
		KeyConditionExpression:    aws.String(keyConditionExpression),
		ExpressionAttributeValues: expressionAttributeValues,
		ProjectionExpression:      aws.String(projectionExpression),
	}

	log.Printf("Executando Query na HouseholdMeasures para food_id: '%s'", foodID)

	result, err := r.DB.Query(ctx, queryInput)
	if err != nil {
		log.Printf("AVISO: Erro ao buscar medidas caseiras para food_id '%s': %v. Retornando apenas 'Grama'.", foodID, err)
		return items, nil
	}

	var dbMeasures []MeasureItem
	err = attributevalue.UnmarshalListOfMaps(result.Items, &dbMeasures)
	if err != nil {
		log.Printf("AVISO: Erro ao fazer unmarshal das medidas caseiras para food_id '%s': %v. Retornando apenas 'Grama'.", foodID, err)
		return items, nil
	}

	items = append(items, dbMeasures...)

	log.Printf("Medidas encontradas para food_id '%s': %d", foodID, len(items))

	return items, nil
}