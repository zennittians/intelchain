package node

import (
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/zennittians/intelchain/consensus"
	"github.com/zennittians/intelchain/consensus/quorum"
	"github.com/zennittians/intelchain/core/types"
	"github.com/zennittians/intelchain/crypto/bls"
	nodeconfig "github.com/zennittians/intelchain/internal/configs/node"
	"github.com/zennittians/intelchain/internal/utils"
	"github.com/zennittians/intelchain/multibls"
	"github.com/zennittians/intelchain/p2p"
	"github.com/zennittians/intelchain/shard"
	staking "github.com/zennittians/intelchain/staking/types"
)

func TestAddNewBlock(t *testing.T) {
	blsKey := bls.RandPrivateKey()
	pubKey := blsKey.GetPublicKey()
	leader := p2p.Peer{IP: "127.0.0.1", Port: "9882", ConsensusPubKey: pubKey}
	priKey, _, _ := utils.GenKeyP2P("127.0.0.1", "9902")
	host, err := p2p.NewHost(&leader, priKey)
	if err != nil {
		t.Fatalf("newhost failure: %v", err)
	}
	decider := quorum.NewDecider(
		quorum.SuperMajorityVote, shard.BeaconChainShardID,
	)
	consensus, err := consensus.New(
		host, shard.BeaconChainShardID, leader, multibls.GetPrivateKeys(blsKey), decider,
	)
	if err != nil {
		t.Fatalf("Cannot craeate consensus: %v", err)
	}
	nodeconfig.SetNetworkType(nodeconfig.Devnet)
	node := New(host, consensus, testDBFactory, nil, nil)

	txs := make(map[common.Address]types.Transactions)
	stks := staking.StakingTransactions{}
	node.Worker.CommitTransactions(
		txs, stks, common.Address{},
	)
	commitSigs := make(chan []byte)
	go func() {
		commitSigs <- []byte{}
	}()
	block, _ := node.Worker.FinalizeNewBlock(
		commitSigs, func() uint64 { return 0 }, common.Address{}, nil, nil,
	)

	_, err = node.Blockchain().InsertChain([]*types.Block{block}, true)
	if err != nil {
		t.Errorf("error when adding new block %v", err)
	}

	if node.Blockchain().CurrentBlock().NumberU64() != 1 {
		t.Error("New block is not added successfully")
	}
}

func TestVerifyNewBlock(t *testing.T) {
	blsKey := bls.RandPrivateKey()
	pubKey := blsKey.GetPublicKey()
	leader := p2p.Peer{IP: "127.0.0.1", Port: "8882", ConsensusPubKey: pubKey}
	priKey, _, _ := utils.GenKeyP2P("127.0.0.1", "9902")
	host, err := p2p.NewHost(&leader, priKey)
	if err != nil {
		t.Fatalf("newhost failure: %v", err)
	}
	decider := quorum.NewDecider(
		quorum.SuperMajorityVote, shard.BeaconChainShardID,
	)
	consensus, err := consensus.New(
		host, shard.BeaconChainShardID, leader, multibls.GetPrivateKeys(blsKey), decider,
	)
	if err != nil {
		t.Fatalf("Cannot craeate consensus: %v", err)
	}
	archiveMode := make(map[uint32]bool)
	archiveMode[0] = true
	archiveMode[1] = false
	node := New(host, consensus, testDBFactory, nil, archiveMode)

	txs := make(map[common.Address]types.Transactions)
	stks := staking.StakingTransactions{}
	node.Worker.CommitTransactions(
		txs, stks, common.Address{},
	)
	commitSigs := make(chan []byte)
	go func() {
		commitSigs <- []byte{}
	}()
	block, _ := node.Worker.FinalizeNewBlock(
		commitSigs, func() uint64 { return 0 }, common.Address{}, nil, nil,
	)

	if err := node.VerifyNewBlock(block); err != nil {
		t.Error("New block is not verified successfully:", err)
	}
}
