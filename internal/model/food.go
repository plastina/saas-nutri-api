package model

type Food struct {
	Id            string             `json:"id"`
	Name          string             `json:"name"`
	Source        string  `json:"source"`
	EnergyKcal    float64            `json:"energy_kcal"`
	ProteinG      float64            `json:"protein_g"`
	CarbohydrateG float64            `json:"carbohydrate_g"`
	FatG          float64            `json:"fat_g"`
	FiberG        float64            `json:"fiber_g"`
	HouseholdMeasures []HouseholdMeasure `json:"household_measures"`
}

type HouseholdMeasure struct {
	Name   string  `json:"name"`  
	Grams  float64 `json:"grams"` 
}