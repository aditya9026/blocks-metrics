package models

import (
	"github.com/lib/pq"
)

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

func GetBlock(id string) *Block {
	block := &Block{}
	err := GetDB().Table("blocks").Where("block_height = ?", id).First(block).Error
	if err != nil {
		return nil
	}
	return block
}

func GetBlocks() []*Block {
	blocks := make([]*Block, 0)
	err := GetDB().Table("blocks").Limit(10).Find(&blocks).Error
	if err != nil {
		return nil
	}
	return blocks
}
