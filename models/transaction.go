package models

import (
	"github.com/jinzhu/gorm/dialects/postgres"
)

type Transaction struct {
	Id              int
	BlockId         int
	TransactionHash []byte
	Message         postgres.Jsonb
}

func GetTransaction(hash string) *Transaction {
	transaction := &Transaction{}
	err := GetDB().Table("transactions").Where("transaction_hash=decode(?, 'hex')", hash).First(transaction).Error
	if err != nil {
		return nil
	}
	return transaction
}

func GetTransactions() []*Transaction {
	transaction := make([]*Transaction, 0)
	err := GetDB().Table("transactions").Order("id desc").Limit(10).Find(&transaction).Error
	if err != nil {
		return nil
	}
	return transaction
}
