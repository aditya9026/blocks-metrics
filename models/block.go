package models

import "github.com/lib/pq"

type Block struct {
	BlockHeight uint64
	BlockHash   []byte
	ProposerID  uint64
	Messages    pq.StringArray `gorm:"type:varchar(64)[]"`
	FeeFrac     uint64
	// ParticipantIDs pq.StringArray `gorm:"type:varchar(64)[]"`
	// MissingIDs     pq.StringArray `gorm:"type:varchar(64)[]"`
	// Transactions   []Transaction
}

func GetBlock(height string) *Block {

	block := &Block{}
	err := GetDB().Table("blocks").Where("block_height = ?", height).First(block).Error
	if err != nil {
		return nil
	}
	return block
}
