package models

type Block struct {
	Height         int64
	Hash           []byte
	ProposerID     int64
	ParticipantIDs []int64
	MissingIDs     []int64
	Messages       []uint8
	FeeFrac        uint64
	Transactions   []Transaction
}

func GetBlock(height string) *Block {

	block := &Block{}
	err := GetDB().Table("blocks").Where("block_height = ?", height).First(block).Error
	if err != nil {
		return nil
	}
	return block
}
