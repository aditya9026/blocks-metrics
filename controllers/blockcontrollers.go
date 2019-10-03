package controllers

import (
	"fmt"
	"net/http"

	"github.com/aditya9026/blocks-metrics/models"
	u "github.com/aditya9026/blocks-metrics/utils"
	"github.com/gorilla/mux"
)

var GetBlocksFor = func(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	fmt.Println("================ GetBlocksFor", id)
	data := models.GetBlock(id)
	resp := u.Message(true, "success")
	resp["data"] = data
	u.Respond(w, resp)
}

var GetTransactionsFor = func(w http.ResponseWriter, r *http.Request) {
	fmt.Println("================ GetTransactionsFor")
	id := mux.Vars(r)["id"]
	data := models.GetTransaction(id)
	resp := u.Message(true, "success")
	resp["data"] = data
	u.Respond(w, resp)
}
