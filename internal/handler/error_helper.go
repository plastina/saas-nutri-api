package handler

import (
	"encoding/json"
	"log"
	"net/http"
	"saas-nutri/internal/model"
)

func RespondWithError(w http.ResponseWriter, code int, message string) {
	log.Printf("Respondendo com erro - Status: %d, Mensagem: %s", code, message)
	RespondWithJSON(w, code, model.APIError{StatusCode: code, Message: message})
}


func RespondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	response, err := json.Marshal(payload)
	if err != nil {
		log.Printf("Erro interno ao fazer marshal da resposta JSON: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"statusCode": 500, "message": "Erro interno do servidor ao formatar resposta."}`))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(response)
}