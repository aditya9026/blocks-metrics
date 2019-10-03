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

func GetBlock(fee_frac string) []*Block {
	blocks := make([]*Block, 0)
	err := GetDB().Table("blocks").Limit(10).Where("fee_frac = ?", fee_frac).Find(&blocks).Error
	if err != nil {
		return nil
	}
	return blocks
}
