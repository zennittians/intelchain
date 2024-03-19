package reward

import (
	"math/big"

	"github.com/zennittians/intelchain/crypto/bls"

	"github.com/ethereum/go-ethereum/common"
	"github.com/zennittians/intelchain/shard"
)

// Payout ..
type Payout struct {
	ShardID     uint32
	Addr        common.Address
	NewlyEarned *big.Int
	EarningKey  bls.SerializedPublicKey
}

// CompletedRound ..
type CompletedRound struct {
	Total            *big.Int
	BeaconchainAward []Payout
	ShardChainAward  []Payout
}

// Reader ..
type Reader interface {
	ReadRoundResult() *CompletedRound
	MissingSigners() shard.SlotList
}
