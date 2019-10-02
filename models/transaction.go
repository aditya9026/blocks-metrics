package models

import (
	"github.com/jinzhu/gorm"
)

type Transaction struct {
	gorm.Model
	Id      uint   `json:"id"`
	Hash    []byte `json:"hash"`
	Message string `json:"message"`
}

func GetTransaction(id uint) *Block {

	block := &Block{}
	err := GetDB().Table("transactions").Where("id = ?", id).First(block).Error
	if err != nil {
		return nil
	}
	return block
}
