package models

import (
	"github.com/jinzhu/gorm"
)

type Block struct {
	gorm.Model
	Id         int64  `json:"id"`
	Height     int64  `json:"height"`
	Hash       []byte `json:"hash"`
	ProposerID int64  `json:"proposer_id"`
	// ParticipantIDs []int64 `json:"participant_ids"`
	Messages []uint8 `json:"messages"`
	FeeFrac  uint64  `json:"fee_frac"`
}

func GetBlock(id string) *Block {

	block := &Block{}
	err := GetDB().Table("blocks").Where("id = ?", id).First(block).Error
	if err != nil {
		return nil
	}
	return block
}
