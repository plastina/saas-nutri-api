// Command importoff faz a carga unica de produtos brasileiros do Open Food
// Facts para a tabela TacoFoods, cobrindo industrializados e suplementos (ex.:
// whey protein) que a base TACO nao tem.
//
// Le um dump ja baixado do OFF (CSV/TSV ou JSONL) de um arquivo local; NAO
// chama a API do OFF (que tem rate limit). Ver o README do backend, secao
// "Carga do Open Food Facts", para onde baixar o dump e a atribuicao exigida
// pela licenca ODbL.
//
// A carga importa apenas produtos com pais Brasil e grava cada um como alimento
// de origem "OFF" (nome + marca, kcal e macros por 100 g); demais nutrientes do
// OFF sao descartados, assim como registros que falham nos filtros de
// plausibilidade. E idempotente: a chave e "off-<codigo>", entao rodar de novo
// reescreve o mesmo item sem duplicar e sem tocar em itens TACO nem na
// HouseholdMeasures.
//
// Uso:
//
//	go run ./cmd/importoff -file ./data/pt.openfoodfacts.org.products.csv
//	go run ./cmd/importoff -file ./data/openfoodfacts-products.jsonl -dry-run
package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"saas-nutri/internal/client"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/joho/godotenv"
)

const (
	tacoTable         = "TacoFoods"
	searchTokensTable = "FoodSearchTokens"
	awsRegion         = "sa-east-1"
)

func main() {
	filePath := flag.String("file", "", "caminho do dump local do Open Food Facts (.csv/.tsv ou .jsonl)")
	format := flag.String("format", "", "formato do dump: csv ou jsonl (padrao: pela extensao do arquivo)")
	delimiter := flag.String("delimiter", "\t", "delimitador do CSV (padrao: tab, como no dump oficial do OFF)")
	batchSize := flag.Int("batch", 500, "quantos alimentos acumular antes de cada gravacao em lote")
	dryRun := flag.Bool("dry-run", false, "processa e imprime o resumo sem gravar no DynamoDB")
	flag.Parse()

	if *filePath == "" {
		log.Fatal("uso: go run ./cmd/importoff -file <dump.csv|dump.jsonl> [-format csv|jsonl] [-dry-run]")
	}
	if *batchSize < 1 {
		log.Fatal("-batch deve ser >= 1")
	}
	if len(*delimiter) != 1 {
		log.Fatal("-delimiter deve ser um unico caractere")
	}

	f, err := os.Open(*filePath)
	if err != nil {
		log.Fatalf("erro ao abrir %s: %v", *filePath, err)
	}
	defer f.Close()

	kind := strings.ToLower(*format)
	if kind == "" {
		kind = formatFromExt(*filePath)
	}

	var reader recordReader
	switch kind {
	case "csv", "tsv":
		reader, err = newCSVReader(f, rune((*delimiter)[0]))
	case "jsonl", "ndjson", "json":
		reader = newJSONLReader(f)
	default:
		log.Fatalf("nao foi possivel determinar o formato do dump; passe -format csv ou -format jsonl")
	}
	if err != nil {
		log.Fatalf("erro ao preparar leitura do dump: %v", err)
	}

	ctx := context.Background()

	var repo *client.TacoRepository
	if !*dryRun {
		if err := godotenv.Load(); err != nil {
			log.Println("Aviso: .env nao encontrado; usando credenciais do ambiente.")
		}
		cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(awsRegion))
		if err != nil {
			log.Fatalf("erro ao carregar configuracao AWS: %v", err)
		}
		repo = client.NewTacoRepository(dynamodb.NewFromConfig(cfg), tacoTable, searchTokensTable)
	}

	sum := summary{discarded: make(map[client.DiscardReason]int)}
	batch := make([]client.TacoFoodItem, 0, *batchSize)

	flush := func() error {
		if len(batch) == 0 {
			return nil
		}
		if repo != nil {
			if err := repo.PutFoodsBatch(ctx, batch); err != nil {
				return err
			}
		}
		sum.imported += len(batch)
		batch = batch[:0]
		return nil
	}

	for {
		rec, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatalf("erro ao ler registro do dump: %v", err)
		}
		sum.read++

		item, reason := client.MapOFFRecord(rec)
		if reason != "" {
			sum.discarded[reason]++
			continue
		}
		batch = append(batch, item)
		if len(batch) >= *batchSize {
			if err := flush(); err != nil {
				log.Fatalf("erro ao gravar lote no DynamoDB: %v", err)
			}
		}
	}
	if err := flush(); err != nil {
		log.Fatalf("erro ao gravar ultimo lote no DynamoDB: %v", err)
	}

	sum.print(os.Stdout, *dryRun)
}

// summary acumula os numeros da carga para o resumo final.
type summary struct {
	read      int
	imported  int
	discarded map[client.DiscardReason]int
}

func (s summary) discardedTotal() int {
	total := 0
	for _, n := range s.discarded {
		total += n
	}
	return total
}

func (s summary) print(w io.Writer, dryRun bool) {
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Resumo da carga Open Food Facts")
	fmt.Fprintf(w, "  registros lidos: %d\n", s.read)
	if dryRun {
		fmt.Fprintf(w, "  importaveis:     %d (dry-run: nada gravado)\n", s.imported)
	} else {
		fmt.Fprintf(w, "  importados:      %d\n", s.imported)
	}
	fmt.Fprintf(w, "  descartados:     %d\n", s.discardedTotal())

	reasons := make([]client.DiscardReason, 0, len(s.discarded))
	for r := range s.discarded {
		reasons = append(reasons, r)
	}
	sort.Slice(reasons, func(i, j int) bool {
		if s.discarded[reasons[i]] != s.discarded[reasons[j]] {
			return s.discarded[reasons[i]] > s.discarded[reasons[j]]
		}
		return reasons[i] < reasons[j]
	})
	for _, r := range reasons {
		fmt.Fprintf(w, "    - %s: %d\n", r, s.discarded[r])
	}
}

func formatFromExt(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".csv":
		return "csv"
	case ".tsv":
		return "tsv"
	case ".jsonl", ".ndjson", ".json":
		return "jsonl"
	default:
		return ""
	}
}

// recordReader entrega os registros do dump um a um, devolvendo io.EOF ao fim.
type recordReader interface {
	Next() (client.OFFRecord, error)
}

// --- CSV / TSV ---

type csvReader struct {
	r    *csv.Reader
	cols map[string]int
}

func newCSVReader(f io.Reader, comma rune) (*csvReader, error) {
	cr := csv.NewReader(bufio.NewReaderSize(f, 1<<20))
	cr.Comma = comma
	cr.LazyQuotes = true
	cr.FieldsPerRecord = -1 // o dump do OFF tem linhas com contagem de campos irregular

	header, err := cr.Read()
	if err != nil {
		return nil, fmt.Errorf("erro ao ler cabecalho do CSV: %w", err)
	}
	cols := make(map[string]int, len(header))
	for i, h := range header {
		cols[strings.TrimSpace(strings.ToLower(h))] = i
	}
	for _, required := range []string{"code", "product_name", "countries_tags"} {
		if _, ok := cols[required]; !ok {
			return nil, fmt.Errorf("coluna obrigatoria %q ausente no cabecalho do CSV", required)
		}
	}
	return &csvReader{r: cr, cols: cols}, nil
}

func (c *csvReader) Next() (client.OFFRecord, error) {
	row, err := c.r.Read()
	if err != nil {
		return client.OFFRecord{}, err // inclui io.EOF
	}
	get := func(name string) string {
		i, ok := c.cols[name]
		if !ok || i >= len(row) {
			return ""
		}
		return strings.TrimSpace(row[i])
	}
	return client.OFFRecord{
		Code:              get("code"),
		ProductName:       get("product_name"),
		Brands:            get("brands"),
		Countries:         splitList(get("countries_tags")),
		EnergyKcal100g:    parseOptionalFloat(get("energy-kcal_100g")),
		Proteins100g:      parseOptionalFloat(get("proteins_100g")),
		Carbohydrates100g: parseOptionalFloat(get("carbohydrates_100g")),
		Fat100g:           parseOptionalFloat(get("fat_100g")),
	}, nil
}

// --- JSONL ---

type jsonlReader struct {
	sc *bufio.Scanner
}

func newJSONLReader(f io.Reader) *jsonlReader {
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 1<<20), 32<<20) // produtos do OFF sao grandes; linha ate 32 MB
	return &jsonlReader{sc: sc}
}

func (j *jsonlReader) Next() (client.OFFRecord, error) {
	for j.sc.Scan() {
		line := bytes.TrimSpace(j.sc.Bytes())
		if len(line) == 0 {
			continue
		}
		var p offJSONProduct
		if err := json.Unmarshal(line, &p); err != nil {
			return client.OFFRecord{}, fmt.Errorf("linha JSONL invalida: %w", err)
		}
		return p.toRecord(), nil
	}
	if err := j.sc.Err(); err != nil {
		return client.OFFRecord{}, err
	}
	return client.OFFRecord{}, io.EOF
}

type offJSONProduct struct {
	Code          string            `json:"code"`
	ProductName   string            `json:"product_name"`
	Brands        string            `json:"brands"`
	CountriesTags []string          `json:"countries_tags"`
	Countries     string            `json:"countries"`
	Nutriments    offJSONNutriments `json:"nutriments"`
}

type offJSONNutriments struct {
	EnergyKcal100g    offFloat `json:"energy-kcal_100g"`
	Proteins100g      offFloat `json:"proteins_100g"`
	Carbohydrates100g offFloat `json:"carbohydrates_100g"`
	Fat100g           offFloat `json:"fat_100g"`
}

func (p offJSONProduct) toRecord() client.OFFRecord {
	countries := p.CountriesTags
	if len(countries) == 0 && p.Countries != "" {
		countries = splitList(p.Countries)
	}
	return client.OFFRecord{
		Code:              strings.TrimSpace(p.Code),
		ProductName:       strings.TrimSpace(p.ProductName),
		Brands:            p.Brands,
		Countries:         countries,
		EnergyKcal100g:    p.Nutriments.EnergyKcal100g.ptr(),
		Proteins100g:      p.Nutriments.Proteins100g.ptr(),
		Carbohydrates100g: p.Nutriments.Carbohydrates100g.ptr(),
		Fat100g:           p.Nutriments.Fat100g.ptr(),
	}
}

// offFloat le um nutriente do JSONL do OFF que pode vir como numero, string
// ("12.3"), null ou ausente. Valor nao numerico ou ausente conta como ausente.
type offFloat struct {
	val   float64
	valid bool
}

func (o *offFloat) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	if len(b) == 0 || string(b) == "null" {
		return nil
	}
	if b[0] == '"' {
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}
		if f, err := strconv.ParseFloat(strings.TrimSpace(s), 64); err == nil {
			o.val, o.valid = f, true
		}
		return nil
	}
	var f float64
	if err := json.Unmarshal(b, &f); err != nil {
		return err
	}
	o.val, o.valid = f, true
	return nil
}

func (o offFloat) ptr() *float64 {
	if !o.valid {
		return nil
	}
	v := o.val
	return &v
}

// --- helpers compartilhados ---

func splitList(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func parseOptionalFloat(s string) *float64 {
	if s == "" {
		return nil
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return nil
	}
	return &f
}
