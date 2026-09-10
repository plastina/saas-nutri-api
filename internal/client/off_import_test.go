package client

import "testing"

func fptr(v float64) *float64 { return &v }

// validOFFRecord e um whey protein brasileiro coerente: kcal declarada 400 vs
// calculada 4*80 + 4*8 + 9*6 = 406 (dentro da tolerancia).
func validOFFRecord() OFFRecord {
	return OFFRecord{
		Code:              "7891000100103",
		ProductName:       "Whey Protein Concentrado",
		Brands:            "Growth Supplements",
		Countries:         []string{"en:brazil"},
		EnergyKcal100g:    fptr(400),
		Proteins100g:      fptr(80),
		Carbohydrates100g: fptr(8),
		Fat100g:           fptr(6),
	}
}

func TestMapOFFRecord(t *testing.T) {
	cases := []struct {
		name       string
		mutate     func(*OFFRecord)
		wantReason DiscardReason
	}{
		{"produto valido", nil, ""},
		{"nome vazio", func(r *OFFRecord) { r.ProductName = "   " }, DiscardEmptyName},
		{"sem kcal", func(r *OFFRecord) { r.EnergyKcal100g = nil }, DiscardMissingNutrient},
		{"sem macro", func(r *OFFRecord) { r.Proteins100g = nil }, DiscardMissingNutrient},
		{"macro negativo", func(r *OFFRecord) { r.Fat100g = fptr(-1) }, DiscardNegative},
		{"kcal negativa", func(r *OFFRecord) { r.EnergyKcal100g = fptr(-10) }, DiscardNegative},
		{
			"soma dos macros acima de 100 g",
			func(r *OFFRecord) {
				r.Proteins100g = fptr(50)
				r.Carbohydrates100g = fptr(40)
				r.Fat100g = fptr(20)
			},
			DiscardMacroSum,
		},
		{
			"kcal fora da faixa plausivel",
			func(r *OFFRecord) {
				r.EnergyKcal100g = fptr(950)
				r.Proteins100g = fptr(0)
				r.Carbohydrates100g = fptr(0)
				r.Fat100g = fptr(100)
			},
			DiscardKcalRange,
		},
		{"kcal incoerente com os macros", func(r *OFFRecord) { r.EnergyKcal100g = fptr(50) }, DiscardKcalMismatch},
		{"pais nao Brasil", func(r *OFFRecord) { r.Countries = []string{"en:france"} }, DiscardNotBrazil},
		{"sem codigo", func(r *OFFRecord) { r.Code = "  " }, DiscardNoCode},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := validOFFRecord()
			if c.mutate != nil {
				c.mutate(&r)
			}
			item, reason := MapOFFRecord(r)
			if reason != c.wantReason {
				t.Fatalf("reason = %q, quer %q", reason, c.wantReason)
			}
			if reason != "" {
				return
			}
			if item.FoodID != "off-7891000100103" {
				t.Errorf("FoodID = %q, quer %q", item.FoodID, "off-7891000100103")
			}
			if item.DataSource != SourceOFF {
				t.Errorf("DataSource = %q, quer %q", item.DataSource, SourceOFF)
			}
			if item.OriginalName != "Whey Protein Concentrado (Growth Supplements)" {
				t.Errorf("OriginalName = %q", item.OriginalName)
			}
			if item.EnergyKcal != 400 || item.ProteinG != 80 || item.CarbohydrateG != 8 || item.FatG != 6 {
				t.Errorf("kcal/macros mapeados errado: %+v", item)
			}
			if item.FiberG != 0 {
				t.Errorf("FiberG = %v, quer 0 (nutrientes extra do OFF sao descartados)", item.FiberG)
			}
		})
	}
}

func TestMapOFFRecordDeterministico(t *testing.T) {
	a, ra := MapOFFRecord(validOFFRecord())
	b, rb := MapOFFRecord(validOFFRecord())
	if ra != "" || rb != "" {
		t.Fatalf("registro valido descartado: %q / %q", ra, rb)
	}
	if a != b {
		t.Errorf("mapeamento nao deterministico (quebra idempotencia): %+v vs %+v", a, b)
	}
}

func TestMappedOFFFoodEhTokenizadoPorProdutoEMarca(t *testing.T) {
	item, reason := MapOFFRecord(validOFFRecord())
	if reason != "" {
		t.Fatalf("registro valido descartado: %q", reason)
	}
	got := make(map[string]bool)
	for _, ti := range buildFoodTokenItems(item) {
		got[ti.Token] = true
	}
	for _, want := range []string{"whey", "protein", "concentrado", "growth", "supplements"} {
		if !got[want] {
			t.Errorf("token %q ausente; tokens = %v", want, got)
		}
	}
}

func TestComposeFoodName(t *testing.T) {
	cases := []struct {
		product, brands, want string
	}{
		{"Whey Protein", "Growth Supplements", "Whey Protein (Growth Supplements)"},
		{"Whey Protein", "", "Whey Protein"},
		{"Whey Protein Growth", "Growth", "Whey Protein Growth"},
		{"Barra de Cereais", "Trio, Nestlé", "Barra de Cereais (Trio)"},
	}
	for _, c := range cases {
		if got := composeFoodName(c.product, c.brands); got != c.want {
			t.Errorf("composeFoodName(%q, %q) = %q, quer %q", c.product, c.brands, got, c.want)
		}
	}
}

func TestIsBrazil(t *testing.T) {
	yes := [][]string{
		{"en:brazil"},
		{"Brazil"},
		{"Brasil"},
		{"pt:brasil"},
		{"en:france", "en:brazil"},
	}
	for _, in := range yes {
		if !isBrazil(in) {
			t.Errorf("isBrazil(%v) = false, quer true", in)
		}
	}
	no := [][]string{
		nil,
		{"en:france"},
		{""},
		{"en:brazil-nuts"},
	}
	for _, in := range no {
		if isBrazil(in) {
			t.Errorf("isBrazil(%v) = true, quer false", in)
		}
	}
}
