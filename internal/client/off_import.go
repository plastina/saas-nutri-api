package client

import (
	"fmt"
	"math"
	"strings"
)

// SourceOFF e a origem gravada em data_source para alimentos vindos do
// Open Food Facts. Distingue-os dos itens da base TACO.
const SourceOFF = "OFF"

// offIDPrefix isola o espaco de chaves dos alimentos importados do OFF dos itens
// TACO (que usam ids numericos). Garante que a carga nunca sobrescreve um item
// TACO e que rodar a carga de novo apenas reescreve o mesmo alimento OFF, sem
// duplicar (a chave e deterministica: offIDPrefix + codigo de barras do OFF).
const offIDPrefix = "off-"

// Limites de plausibilidade da carga OFF, por 100 g. A tolerancia entre a kcal
// declarada e a recalculada pelos macros (4/4/9) e frouxa de proposito: fibra,
// polialcoois e o arredondamento do proprio OFF explicam alguma diferenca; o
// objetivo e so barrar registro grosseiramente inconsistente.
const (
	maxKcalPer100g   = 900.0
	maxMacroSumG     = 100.0
	kcalToleranceAbs = 50.0 // diferenca absoluta tolerada, em kcal
	kcalToleranceRel = 0.25 // diferenca relativa tolerada (fracao da kcal calculada)
)

// DiscardReason identifica por que um registro do OFF nao virou alimento. Serve
// tambem de chave na contagem por motivo no resumo final da carga.
type DiscardReason string

const (
	DiscardNotBrazil       DiscardReason = "país não é Brasil"
	DiscardNoCode          DiscardReason = "código OFF ausente"
	DiscardEmptyName       DiscardReason = "nome do produto vazio"
	DiscardMissingNutrient DiscardReason = "kcal ou macro ausente"
	DiscardNegative        DiscardReason = "kcal ou macro negativo"
	DiscardMacroSum        DiscardReason = "soma dos macros acima de 100 g"
	DiscardKcalRange       DiscardReason = "kcal fora da faixa plausível"
	DiscardKcalMismatch    DiscardReason = "kcal declarada incoerente com os macros"
)

// OFFRecord e um registro cru do dump do Open Food Facts, com apenas os campos
// que a carga usa ja extraidos do CSV/JSONL. Um nutriente nil significa ausente
// no dump (diferente de zero declarado).
type OFFRecord struct {
	Code              string
	ProductName       string
	Brands            string
	Countries         []string // tags ou nomes de pais, como vierem no dump
	EnergyKcal100g    *float64
	Proteins100g      *float64
	Carbohydrates100g *float64
	Fat100g           *float64
}

// MapOFFRecord aplica os filtros da carga e, se o registro passar, devolve o
// alimento pronto para gravar (origem OFF, valores por 100 g). Um reason != ""
// indica registro descartado (o TacoFoodItem devolvido fica zerado); reason ""
// indica item valido.
func MapOFFRecord(r OFFRecord) (TacoFoodItem, DiscardReason) {
	if !isBrazil(r.Countries) {
		return TacoFoodItem{}, DiscardNotBrazil
	}

	code := strings.TrimSpace(r.Code)
	if code == "" {
		return TacoFoodItem{}, DiscardNoCode
	}

	name := strings.TrimSpace(r.ProductName)
	if name == "" {
		return TacoFoodItem{}, DiscardEmptyName
	}

	if r.EnergyKcal100g == nil || r.Proteins100g == nil || r.Carbohydrates100g == nil || r.Fat100g == nil {
		return TacoFoodItem{}, DiscardMissingNutrient
	}
	kcal, protein, carb, fat := *r.EnergyKcal100g, *r.Proteins100g, *r.Carbohydrates100g, *r.Fat100g

	if kcal < 0 || protein < 0 || carb < 0 || fat < 0 {
		return TacoFoodItem{}, DiscardNegative
	}
	if protein+carb+fat > maxMacroSumG {
		return TacoFoodItem{}, DiscardMacroSum
	}
	if kcal > maxKcalPer100g {
		return TacoFoodItem{}, DiscardKcalRange
	}

	calculated := 4*protein + 4*carb + 9*fat
	if diff := math.Abs(kcal - calculated); diff > kcalToleranceAbs && diff > kcalToleranceRel*calculated {
		return TacoFoodItem{}, DiscardKcalMismatch
	}

	display := composeFoodName(name, r.Brands)
	return TacoFoodItem{
		FoodID:         offIDPrefix + code,
		DataSource:     SourceOFF,
		OriginalName:   display,
		NormalizedName: display,
		EnergyKcal:     kcal,
		ProteinG:       protein,
		CarbohydrateG:  carb,
		FatG:           fat,
		FiberG:         0,
	}, ""
}

// isBrazil reconhece o pais Brasil em qualquer forma que o OFF usa: tag
// canonica ("en:brazil"), nome em ingles ("Brazil") ou em portugues ("Brasil"),
// com ou sem prefixo de idioma.
func isBrazil(countries []string) bool {
	for _, c := range countries {
		c = strings.ToLower(strings.TrimSpace(c))
		if i := strings.LastIndex(c, ":"); i >= 0 {
			c = c[i+1:]
		}
		if c == "brazil" || c == "brasil" {
			return true
		}
	}
	return false
}

// composeFoodName monta o nome exibido e buscado do alimento combinando produto
// e marca. A marca entra entre parenteses; como a normalizacao da busca
// (NormalizeTokens) trata parenteses como separador, as palavras da marca viram
// tokens e o alimento e achado tanto pelo produto quanto pela marca. Marca
// vazia, ou ja contida no nome do produto, e ignorada.
func composeFoodName(product, brands string) string {
	brand := firstBrand(brands)
	if brand == "" || strings.Contains(strings.ToLower(product), strings.ToLower(brand)) {
		return product
	}
	return fmt.Sprintf("%s (%s)", product, brand)
}

// firstBrand devolve a primeira marca de uma lista separada por virgula do OFF
// (ex.: "Growth,Growth Supplements" -> "Growth").
func firstBrand(brands string) string {
	for _, b := range strings.Split(brands, ",") {
		if b = strings.TrimSpace(b); b != "" {
			return b
		}
	}
	return ""
}
