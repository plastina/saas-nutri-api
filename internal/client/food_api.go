package client

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"saas-nutri/internal/model"
	"strconv"
	"time"
)

type FoodAPIClient interface {
	SearchFoods(query string) ([]model.Food, error)
}


const offBaseURL = "https://world.openfoodfacts.org/cgi/search.pl"

type offResponse struct {
	Products []offProduct `json:"products"`
	Count    int          `json:"count"`
}

type offProduct struct {
	ID          string        `json:"_id"`
	ProductName string        `json:"product_name"`
	Nutriments  offNutriments `json:"nutriments"`
}

type offNutriments struct {
	EnergyKcal100g  interface{} `json:"energy-kcal_100g"`
	Proteins100g    interface{} `json:"proteins_100g"`
	Carbohyates100g interface{} `json:"carbohydrates_100g"`
	Fat100g         interface{} `json:"fat_100g"`
	Fiber100g       interface{} `json:"fiber_100g"`
}

type offClient struct {
	httpClient *http.Client
	baseURL    string
}

func NewOpenFoodFactsClient() FoodAPIClient {
	return &offClient{
		httpClient: &http.Client{Timeout: 10 * time.Second},
		baseURL:    offBaseURL,
	}
}

func (c *offClient) SearchFoods(query string) ([]model.Food, error) {
	apiURL, _ := url.Parse(c.baseURL)
	params := url.Values{}
	params.Add("search_terms", query)
	params.Add("search_simple", "1")
	params.Add("action", "process")
	params.Add("json", "1")                         
	params.Add("page_size", "10")                   
	params.Add("fields", "product_name,nutriments") 
	apiURL.RawQuery = params.Encode()

	req, err := http.NewRequest("GET", apiURL.String(), nil)
	if err != nil {
		log.Printf("Erro ao criar request para OFF: %v", err)
		return nil, fmt.Errorf("erro interno ao preparar busca")
	}
	req.Header.Set("User-Agent", "SaaS Nutri MVP - Golang Client - v0.1")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		log.Printf("Erro ao fazer request para OFF: %v", err)
		return nil, fmt.Errorf("erro ao conectar com API de alimentos")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("Erro status code da OFF API: %d", resp.StatusCode)
		return nil, fmt.Errorf("API de alimentos retornou erro %d", resp.StatusCode)
	}

	var offResp offResponse
	if err := json.NewDecoder(resp.Body).Decode(&offResp); err != nil {
		log.Printf("Erro ao decodificar JSON da OFF: %v", err)
		return nil, fmt.Errorf("erro ao ler resposta da API de alimentos")
	}

	foods := make([]model.Food, 0, len(offResp.Products))
	for _, p := range offResp.Products {
		if p.ProductName == "" { 
			continue
		}
		foods = append(foods, model.Food{
			ID:            p.ID,
			Name:          p.ProductName,
			Source:        "OpenFoodFacts",
			EnergyKcal:    parseFloatOrZero(p.Nutriments.EnergyKcal100g),
			ProteinG:      parseFloatOrZero(p.Nutriments.Proteins100g),
			CarbohydrateG: parseFloatOrZero(p.Nutriments.Carbohyates100g),
			FatG:          parseFloatOrZero(p.Nutriments.Fat100g),
			FiberG:        parseFloatOrZero(p.Nutriments.Fiber100g),
		})
	}

	return foods, nil
}

func parseFloatOrZero(value interface{}) float64 {
	if value == nil {
		return 0
	}
	switch v := value.(type) {
	case float64:
		return v
	case string:
		f, err := strconv.ParseFloat(v, 64)
		if err == nil {
			return f
		}
	case int:
		return float64(v)
	case int64:
		return float64(v)
	}
	return 0
}

type mockFoodAPIClient struct{}

func NewMockFoodAPIClient() FoodAPIClient {
	return &mockFoodAPIClient{}
}

func (m *mockFoodAPIClient) SearchFoods(query string) ([]model.Food, error) {
	log.Printf("Cliente Mock: Buscando por '%s'\n", query)
	if query == "maçã" {
		return []model.Food{
			{ID: "mock1", Name: "Maçã Fuji (Mock)", Source: "mock", EnergyKcal: 52, ProteinG: 0.3, CarbohydrateG: 14, FatG: 0.2, FiberG: 2.4},
			{ID: "mock2", Name: "Maçã Gala (Mock)", Source: "mock", EnergyKcal: 57, ProteinG: 0.2, CarbohydrateG: 13.7, FatG: 0.1, FiberG: 2.1},
		}, nil
	}
	return []model.Food{}, nil
}
