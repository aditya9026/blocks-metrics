package controllers

import (
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/iov-one/block-metrics/models"
	u "github.com/iov-one/block-metrics/utils"
)

var GetBlocksFor = func(w http.ResponseWriter, r *http.Request) {
	fmt.Println("================")
	id := mux.Vars(r)["id"]
	data := models.GetBlock(id)
	resp := u.Message(true, "success")
	resp["data"] = data
	u.Respond(w, resp)
}
