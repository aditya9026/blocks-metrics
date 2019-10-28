package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/iov-one/blocks-metrics/controllers"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
)

func main() {

	router := mux.NewRouter()

	router.HandleFunc("/api/last_records", controllers.LastRecords).Methods("GET")
	router.HandleFunc("/api/block/{hash:[0-9a-fA-F]+}", controllers.GetBlockFor).Methods("GET")
	router.HandleFunc("/api/transaction/{hash:[0-9a-fA-F]+}", controllers.GetTransactionFor).Methods("GET")

	router.HandleFunc("/api/blocks", controllers.GetBlocksFor).Methods("GET")
	router.HandleFunc("/api/transactions", controllers.GetTransactionsFor).Methods("GET")

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
