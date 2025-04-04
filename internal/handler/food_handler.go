package handler

import (
	"encoding/json"
	"log"
	"net/http"
	"saas-nutri/internal/client"
)

type FoodHandler struct {
	apiClient client.FoodAPIClient
}

func NewFoodHandler(apiClient client.FoodAPIClient) *FoodHandler {
	return &FoodHandler{
		apiClient: apiClient,
	}
}

// SearchFoods godoc
// @Summary      Busca alimentos
// @Description  Busca alimentos na base de dados (mock/API externa) com base em um termo de pesquisa.
// @Tags         alimentos
// @Accept       json
// @Produce      json
// @Param        search query string true "Termo para buscar o alimento" example(maçã)
// @Success      200 {array} model.Food "Lista de alimentos encontrados"
// @Failure      400 {object} string "Erro pela falta do parâmetro 'search'"
// @Failure      500 {object} string "Erro interno ao buscar os alimentos"
// @Router       /foods [get]
func (h *FoodHandler) SearchFoods(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("search")
	if query == "" {
		http.Error(w, "Parâmetro 'search' é obrigatório", http.StatusBadRequest)
		return
	}

	log.Printf("Handler: Recebida busca por '%s'\n", query)

	foods, err := h.apiClient.SearchFoods(query) 

	if err != nil {
		log.Printf("Erro ao buscar alimentos da API externa: %v\n", err)
		http.Error(w, "Erro ao buscar dados dos alimentos", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(foods); err != nil {
		log.Printf("Erro ao codificar resposta JSON: %v\n", err)
	}
}
