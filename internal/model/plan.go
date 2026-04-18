package model

type MealItemInput struct {
	FoodID          string  `json:"food_id"`
	QuantityInGrams float64 `json:"quantity_in_grams,omitempty"`
	PortionSize     string  `json:"portion_size,omitempty"`
}

type MealInput struct {
	Name  string          `json:"name"`
	Items []MealItemInput `json:"items"`
}

type PlanCalculateRequest struct {
	Meals []MealInput `json:"meals"`
}

type MealItemCalories struct {
	FoodID          string  `json:"food_id"`
	Name            string  `json:"name"`
	QuantityInGrams float64 `json:"quantity_in_grams"`
	Kcal            float64 `json:"kcal"`
}

type MealCalories struct {
	Name      string             `json:"name"`
	Items     []MealItemCalories `json:"items"`
	TotalKcal float64            `json:"total_kcal"`
}

type PlanCalculateResponse struct {
	Meals     []MealCalories `json:"meals"`
	TotalKcal float64        `json:"total_kcal"`
}
