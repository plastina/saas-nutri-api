package handler

import (
	"encoding/json"
	"log"
	"net/http"

	"saas-nutri/internal/client"
	"saas-nutri/internal/model"

	"github.com/go-chi/chi/v5"
)

type FoodHandler struct {
	tacoRepo *client.TacoRepository
}

func NewFoodHandler(taco *client.TacoRepository) *FoodHandler {
	return &FoodHandler{
		tacoRepo: taco,
	}
}

func mapTacoToFood(tacoItem client.TacoFoodItem) model.Food {
	return model.Food{
		Id:            tacoItem.FoodID,
		Name:          tacoItem.OriginalName,
		Source:        "TACO",
		EnergyKcal:    tacoItem.EnergyKcal,
		ProteinG:      tacoItem.ProteinG,
		CarbohydrateG: tacoItem.CarbohydrateG,
		FatG:          tacoItem.FatG,
		FiberG:        tacoItem.FiberG,
	}
}

// SearchFoods godoc
// @Summary      Busca alimentos
// @Description  Busca alimentos na base TACO
// @Tags         alimentos
// @Accept       json
// @Produce      json
// @Param        search query string true "Termo para buscar o alimento" example(arroz)
// @Success      200 {array} model.Food "Lista de alimentos encontrados da TACO"
// @Failure      400 {object} string "Erro: Parâmetro 'search' é obrigatório"
// @Failure      500 {object} string "Erro interno ao buscar dados dos alimentos"
// @Router       /foods [get]

func (h *FoodHandler) SearchFoods(w http.ResponseWriter, r *http.Request) {
	searchTerm := r.URL.Query().Get("search")
	if searchTerm == "" {
		http.Error(w, "Parâmetro 'search' é obrigatório", http.StatusBadRequest)
		return
	}

	log.Printf("Handler (TACO Only): Recebida busca por '%s'", searchTerm)
	ctx := r.Context()

	var mappedResults []model.Food

	tacoResults, errTaco := h.tacoRepo.SearchFoodsByNamePrefix(ctx, searchTerm)
	if errTaco != nil {
		log.Printf("ERRO ao buscar na TACO (DynamoDB): %v", errTaco)
		http.Error(w, "Erro interno ao buscar dados dos alimentos", http.StatusInternalServerError)
		return
	}

	for _, tacoItem := range tacoResults {
		mappedTacoItem := mapTacoToFood(tacoItem)
		mappedResults = append(mappedResults, mappedTacoItem)
	}

	log.Printf("Handler (TACO Only): Retornando %d resultados para '%s'", len(mappedResults), searchTerm)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(mappedResults); err != nil {
		log.Printf("Erro ao codificar resposta JSON: %v", err)
	}
}

// GetFoodMeasures godoc
// @Summary      Busca medidas caseiras de um alimento
// @Description  Retorna uma lista de medidas caseiras e seus equivalentes em gramas para um ID de alimento específico.
// @Tags         alimentos
// @Accept       json
// @Produce      json
// @Param        foodId path string true "ID do Alimento (ex: UUID ou código TACO)"
// @Success      200 {array} client.MeasureItem "Lista de medidas caseiras"
// @Failure      400 {object} string "Erro: ID do alimento é obrigatório"
// @Failure      500 {object} string "Erro interno ao buscar medidas"
// @Router       /foods/{foodId}/measures [get]

func (h *FoodHandler) GetFoodMeasures(w http.ResponseWriter, r *http.Request) {
	foodId := chi.URLParam(r, "foodId")
	if foodId == "" {
		http.Error(w, "ID do alimento é obrigatório na URL", http.StatusBadRequest)
		return
	}

	log.Printf("Handler: Recebida busca por medidas para foodId '%s'", foodId)
	ctx := r.Context()

	measures, err := h.tacoRepo.GetMeasuresForFood(ctx, foodId)
	if err != nil {
		log.Printf("Erro (já logado) ao buscar medidas para foodId '%s', retornando o que foi possível.", foodId)
	}

	if measures == nil {
		measures = []client.MeasureItem{
			{MeasureName: "grama", DisplayName: "Grama", GramEquivalent: 1.0},
		}
	}

	log.Printf("Handler: Retornando %d medidas para foodId '%s'", len(measures), foodId)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(measures); err != nil {
		log.Printf("Erro ao encodar resposta JSON de medidas: %v", err)
	}
}