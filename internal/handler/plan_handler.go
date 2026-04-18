package handler

import (
	"encoding/json"
	"math"
	"net/http"

	"saas-nutri/internal/client"
	"saas-nutri/internal/model"
)

type PlanHandler struct {
	tacoRepo *client.TacoRepository
}

func NewPlanHandler(taco *client.TacoRepository) *PlanHandler {
	return &PlanHandler{tacoRepo: taco}
}

// CalculateMealCalories godoc
// @Summary      Calcula calorias por refeicao
// @Description  Recebe um plano com refeicoes e itens (food_id e quantidade em gramas) e retorna kcal por item e total por refeicao.
// @Tags         plano
// @Accept       json
// @Produce      json
// @Param        payload body model.PlanCalculateRequest true "Plano com refeicoes"
// @Success      200 {object} model.PlanCalculateResponse "Totais calculados"
// @Failure      400 {object} string "Erro de validacao"
// @Failure      404 {object} string "Alimento nao encontrado"
// @Failure      500 {object} string "Erro interno"
// @Router       /plan/calculate [post]
func (h *PlanHandler) CalculateMealCalories(w http.ResponseWriter, r *http.Request) {
	var request model.PlanCalculateRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		RespondWithError(w, http.StatusBadRequest, "Corpo JSON invalido")
		return
	}
	if len(request.Meals) == 0 {
		RespondWithError(w, http.StatusBadRequest, "Plano sem refeicoes")
		return
	}

	foodCache := make(map[string]*model.Food)
	response := model.PlanCalculateResponse{
		Meals: []model.MealCalories{},
	}

	for _, meal := range request.Meals {
		mealResult := model.MealCalories{
			Name:  meal.Name,
			Items: []model.MealItemCalories{},
		}
		if len(meal.Items) == 0 {
			response.Meals = append(response.Meals, mealResult)
			continue
		}

		for _, item := range meal.Items {
			if item.FoodID == "" {
				RespondWithError(w, http.StatusBadRequest, "Item sem food_id")
				return
			}

			food, ok := foodCache[item.FoodID]
			if !ok {
				found, err := h.tacoRepo.GetFoodWithMeasures(r.Context(), item.FoodID)
				if err != nil {
					RespondWithError(w, http.StatusNotFound, "Alimento nao encontrado")
					return
				}
				food = found
				foodCache[item.FoodID] = food
			}

			quantity := item.QuantityInGrams
			if item.PortionSize != "" {
				// Encontrar o PortionSize correspondente
				found := false
				for _, ps := range food.PortionSizes {
					if ps.Size == item.PortionSize {
						quantity = ps.Grams
						found = true
						break
					}
				}
				if !found {
					RespondWithError(w, http.StatusBadRequest, "Tamanho de porcao invalido para o alimento")
					return
				}
			}

			quantityFactor := quantity / 100
			kcal := (food.EnergyKcal * quantityFactor)
			kcal = roundToDecimals(kcal, 2)

			mealResult.Items = append(mealResult.Items, model.MealItemCalories{
				FoodID:          item.FoodID,
				Name:            food.Name,
				QuantityInGrams: quantity,
				Kcal:            kcal,
			})
			mealResult.TotalKcal += kcal
		}

		mealResult.TotalKcal = roundToDecimals(mealResult.TotalKcal, 2)
		response.Meals = append(response.Meals, mealResult)
		response.TotalKcal += mealResult.TotalKcal
	}

	response.TotalKcal = roundToDecimals(response.TotalKcal, 2)
	RespondWithJSON(w, http.StatusOK, response)
}

func roundToDecimals(value float64, places int) float64 {
	if places < 0 {
		return value
	}
	multiplier := math.Pow(10, float64(places))
	return math.Round(value*multiplier) / multiplier
}
