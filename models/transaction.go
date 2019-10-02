package models

type Transaction struct {
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
