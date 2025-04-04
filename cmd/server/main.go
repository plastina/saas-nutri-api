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
	"log"
	"net/http"
	_ "saas-nutri/docs"
	"saas-nutri/internal/client"
	"saas-nutri/internal/handler"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	httpSwagger "github.com/swaggo/http-swagger"
)

func main() {
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
	apiClient := client.NewOpenFoodFactsClient() 
	foodHandler := handler.NewFoodHandler(apiClient)

	r.Get("/swagger/*", httpSwagger.WrapHandler)

	r.Route("/api", func(r chi.Router) {
		r.Get("/foods", foodHandler.SearchFoods)
	})

	log.Println("Servidor iniciado na porta :8080")
	log.Println("Swagger UI disponível em http://localhost:8080/swagger/index.html") 
	err := http.ListenAndServe(":8080", r)
	if err != nil {
		log.Fatalf("Erro ao iniciar o servidor: %v", err)
	}
}
