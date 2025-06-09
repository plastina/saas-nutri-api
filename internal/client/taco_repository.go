package client

import (
	"context"
	"fmt"
	"log"
	"saas-nutri/internal/model"
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
    FoodID           string  `json:"-" dynamodbav:"food_id"`
    MeasureName      string  `json:"measure_name" dynamodbav:"measure_name"`
    MeasureQuantity  string  `json:"measure_quantity" dynamodbav:"measure_quantity"`
    DisplayName      string  `json:"display_name"`
    GramEquivalent   float64 `json:"gram_equivalent" dynamodbav:"measure_weight_g"`
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
    
    projectionExpression := "food_id, measure_name, measure_quantity, measure_weight_g"

    log.Printf("Buscando medidas para food_id: %s", foodID)

    queryInput := &dynamodb.QueryInput{
        TableName:                 aws.String("HouseholdMeasures"),
        KeyConditionExpression:    aws.String(keyConditionExpression),
        ExpressionAttributeValues: expressionAttributeValues,
        ProjectionExpression:      aws.String(projectionExpression),
    }

    result, err := r.DB.Query(ctx, queryInput)
    if err != nil {
        log.Printf("ERRO ao buscar medidas: %v", err)
        return nil, fmt.Errorf("erro ao buscar medidas no DB para food_id %s: %w", foodID, err)
    }

    log.Printf("DynamoDB retornou %d itens", len(result.Items))
    for i, item := range result.Items {
        log.Printf("Item %d: %+v", i, item)
    }

    var dbMeasures []MeasureItem
    err = attributevalue.UnmarshalListOfMaps(result.Items, &dbMeasures)
    if err != nil {
        log.Printf("ERRO no unmarshal: %v", err)
        return nil, fmt.Errorf("erro ao processar medidas do DB para food_id %s: %w", foodID, err)
    }

    log.Printf("Após unmarshal: %+v", dbMeasures)

    for i := range dbMeasures {
        if dbMeasures[i].MeasureQuantity != "" {
            dbMeasures[i].DisplayName = dbMeasures[i].MeasureQuantity + " " + dbMeasures[i].MeasureName
        } else {
            dbMeasures[i].DisplayName = dbMeasures[i].MeasureName
        }
        log.Printf("Medida %d: name=%s, quantity=%s, weight=%f, display=%s", 
            i, dbMeasures[i].MeasureName, dbMeasures[i].MeasureQuantity, 
            dbMeasures[i].GramEquivalent, dbMeasures[i].DisplayName)
    }

    testItem := MeasureItem{
        FoodID:          foodID,
        MeasureName:     "colher de sopa",
        MeasureQuantity: "1",
        DisplayName:     "1 colher de sopa",
        GramEquivalent:  15.0,
    }
    items = append(items, testItem)
    
    items = append(items, dbMeasures...)
    return items, nil
}

func (r *TacoRepository) GetFoodWithMeasures(ctx context.Context, foodID string) (*model.Food, error) {
	key := map[string]types.AttributeValue{
		"food_id": &types.AttributeValueMemberS{Value: foodID},
	}

	getItemInput := &dynamodb.GetItemInput{
		TableName: aws.String(r.TableName),
		Key:       key,
	}

	result, err := r.DB.GetItem(ctx, getItemInput)
	if err != nil {
		return nil, fmt.Errorf("erro ao buscar alimento no DynamoDB: %w", err)
	}

	if result.Item == nil {
		return nil, fmt.Errorf("alimento não encontrado: %s", foodID)
	}

	var foodItem TacoFoodItem
	err = attributevalue.UnmarshalMap(result.Item, &foodItem)
	if err != nil {
		return nil, fmt.Errorf("erro ao deserializar alimento: %w", err)
	}

	food := &model.Food{
		Id:            foodItem.FoodID,
		Name:          foodItem.NormalizedName, 
		Source:        foodItem.DataSource,
		EnergyKcal:    foodItem.EnergyKcal,
		ProteinG:      foodItem.ProteinG,
		CarbohydrateG: foodItem.CarbohydrateG,
		FatG:          foodItem.FatG,
		FiberG:        foodItem.FiberG,
	}

	measures, err := r.GetMeasuresForFood(ctx, foodID)
	if err != nil {
		return nil, fmt.Errorf("erro ao buscar medidas caseiras: %w", err)
	}

	var householdMeasures []model.HouseholdMeasure
	for _, m := range measures {
		householdMeasures = append(householdMeasures, model.HouseholdMeasure{
			Name:  m.DisplayName,
			Grams: m.GramEquivalent,
		})
	}

	food.HouseholdMeasures = householdMeasures

	return food, nil
}