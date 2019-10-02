package main

import (
	"fmt"
	"net/http"
	"os"

	"github.com/gorilla/mux"
	"github.com/iov-one/block-metrics/controllers"
)

func main() {

	router := mux.NewRouter()

	router.HandleFunc("/api/blocks/{id:[0-9]+}", controllers.GetBlocksFor).Methods("GET")
	// router.NotFoundHandler = app.NotFoundHandler

	port := os.Getenv("PORT")
	if port == "" {
		port = "3000" //localhost
	}

	fmt.Println("port", port)

	err := http.ListenAndServe(":"+port, router) //Launch the app, visit localhost:3000/api
	if err != nil {
		fmt.Print(err)
	}
}
