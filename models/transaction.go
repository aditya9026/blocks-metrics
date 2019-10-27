package models

type Transaction struct {
	Id              int
	BlockId         int
	TransactionHash []byte
	Message         string
}

func GetTransaction(id string) *Transaction {
	transaction := &Transaction{}
	err := GetDB().Table("transactions").Where("id = ?", id).First(transaction).Error
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
