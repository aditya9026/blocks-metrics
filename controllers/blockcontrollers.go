package controllers

import (
	"fmt"
	"net/http"

	"github.com/aditya9026/blocks-metrics/models"
	u "github.com/aditya9026/blocks-metrics/utils"
	"github.com/gorilla/mux"
)

var GetBlockFor = func(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	fmt.Println("================ GetBlockFor", id)
	data := models.GetBlock(id)
	resp := u.Message(true, "success")
	resp["block"] = data
	u.Respond(w, resp)
}

var GetBlocksFor = func(w http.ResponseWriter, r *http.Request) {
	fmt.Println("================ GetBlocksFor last 10")
	data := models.GetBlocks()
	resp := u.Message(true, "success")
	resp["blocks"] = data
	u.Respond(w, resp)
}

var GetTransactionFor = func(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	fmt.Println("================ GetTransactionFor", id)
	data := models.GetTransaction(id)
	resp := u.Message(true, "success")
	resp["transaction"] = data
	u.Respond(w, resp)
}

var GetTransactionsFor = func(w http.ResponseWriter, r *http.Request) {
	fmt.Println("================ GetTransactionsFor last 10")
	data := models.GetTransactions()
	resp := u.Message(true, "success")
	resp["transactions"] = data
	u.Respond(w, resp)
}
