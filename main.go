package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/aditya9026/blocks-metrics/controllers"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
)

func main() {

	router := mux.NewRouter()

	router.HandleFunc("/api/blocks/{id:[0-9]+}", controllers.GetBlocksFor).Methods("GET")
	router.HandleFunc("/api/transactions/{id:[0-9]+}", controllers.GetTransactionsFor).Methods("GET")
	corsObj := handlers.AllowedOrigins([]string{"*"})
	// router.NotFoundHandler = app.NotFoundHandler

	port := os.Getenv("PORT")
	if port == "" {
		port = "3001" //localhost
	}

	fmt.Println("port", port)

	log.Fatal(http.ListenAndServe(":"+port, handlers.CORS(corsObj)(router)))
	err := http.ListenAndServe(":"+port, router) //Launch the app, visit localhost:3000/api
	if err != nil {
		fmt.Print(err)
	}
}
