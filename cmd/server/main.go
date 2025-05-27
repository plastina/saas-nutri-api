// @title           API SaaS Nutri
// @version         1.0
// @description     API para o MVP do SaaS para nutricionistas.
// @termsOfService  http://swagger.io/terms/

// @contact.name   API Support
// @contact.url    http://www.swagger.io/support
// @contact.email  support@swagger.io

// @license.name  Apache 2.0
// @license.url   http://www.apache.org/licenses/LICENSE-2.0.html

// @host      localhost:8080
// @BasePath  /api

package main

import (
	"context"
	"log"
	"net/http"
	_ "saas-nutri/docs"
	"saas-nutri/internal/client"
	"saas-nutri/internal/handler"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/joho/godotenv"
	httpSwagger "github.com/swaggo/http-swagger"
)


func main() {
	errEnv := godotenv.Load()
	if errEnv != nil {
		log.Println("Aviso: Arquivo .env não encontrado ou erro ao carregar.")
	} else {
		log.Println("Arquivo .env carregado com sucesso.")
	}


	log.Println("Iniciando configuração da API...")

	r := chi.NewRouter()

	corsMiddleware := cors.New(cors.Options{
		AllowedOrigins:   []string{"http://localhost:4200"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	})
	r.Use(corsMiddleware.Handler)

	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	log.Println("Inicializando dependências...")

	awsRegion := "sa-east-1"
	cfg, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion(awsRegion))
	if err != nil {
		log.Fatalf("PANIC: Erro ao carregar configuração AWS para API: %v", err)
	}
	dynamoClient := dynamodb.NewFromConfig(cfg)
	log.Println("Cliente DynamoDB inicializado na região:", awsRegion)


	tacoTableName := "TacoFoods"
	tacoIndexName := "FoodNameIndex"
	tacoRepo := client.NewTacoRepository(dynamoClient, tacoTableName, tacoIndexName)
	log.Println("Repositório TACO (DynamoDB) inicializado.")


	foodHandler := handler.NewFoodHandler(tacoRepo)
	log.Println("Handler de Alimentos inicializado (modo TACO only).")


	log.Println("Configurando rotas...")

	r.Get("/swagger/*", httpSwagger.WrapHandler)
	log.Println("Rota Swagger configurada em /swagger/*")


	r.Route("/api", func(r chi.Router) {
		log.Println("Configurando rotas sob /api...")

		r.Route("/foods", func(r chi.Router) {
		r.Get("/", foodHandler.SearchFoods)
		log.Println("Rota GET /api/foods configurada.")

		r.Get("/{foodId}", foodHandler.GetFoodWithMeasures)
		log.Println("Rota GET /api/foods/{foodId} configurada.")

		r.Get("/{foodId}/measures", foodHandler.GetFoodMeasures)
		log.Println("Rota GET /api/foods/{foodId}/measures configurada.")
	})


	})


	port := ":8080"
	log.Printf("Servidor pronto para iniciar na porta %s...", port)
	log.Printf("Swagger UI disponível em http://localhost%s/swagger/index.html", port)
	err = http.ListenAndServe(port, r)
	if err != nil {
		log.Fatalf("PANIC: Erro fatal ao iniciar o servidor HTTP na porta %s: %v", port, err)
	}
}