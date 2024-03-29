package node

import (
	"errors"
	"math/big"

	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/zennittians/intelchain/common/denominations"
	"github.com/zennittians/intelchain/core"
	common2 "github.com/zennittians/intelchain/internal/common"
	"github.com/zennittians/intelchain/internal/genesis"
	"github.com/zennittians/intelchain/internal/utils"
	"github.com/zennittians/intelchain/shard"
	"github.com/zennittians/intelchain/shard/committee"
)

// genesisInitializer is a shardchain.DBInitializer adapter.
type genesisInitializer struct {
	node *Node
}

// InitChainDB sets up a new genesis block in the database for the given shard.
func (gi *genesisInitializer) InitChainDB(db ethdb.Database, shardID uint32) error {
	shardState, _ := committee.WithStakingEnabled.Compute(
		big.NewInt(core.GenesisEpoch), nil,
	)
	if shardState == nil {
		return errors.New("failed to create genesis shard state")
	}
	if shardID != shard.BeaconChainShardID {
		// store only the local shard for shard chains
		subComm, err := shardState.FindCommitteeByID(shardID)
		if err != nil {
			return errors.New("cannot find local shard in genesis")
		}
		shardState = &shard.State{nil, []shard.Committee{*subComm}}
	}
	gi.node.SetupGenesisBlock(db, shardID, shardState)
	return nil
}

// SetupGenesisBlock sets up a genesis blockchain.
func (node *Node) SetupGenesisBlock(db ethdb.Database, shardID uint32, myShardState *shard.State) {
	utils.Logger().Info().Interface("shardID", shardID).Msg("setting up a brand new chain database")
	if shardID == node.NodeConfig.ShardID {
		node.isFirstTime = true
	}

	gspec := core.NewGenesisSpec(node.NodeConfig.GetNetworkType(), shardID)
	gspec.ShardStateHash = myShardState.Hash()
	gspec.ShardState = *myShardState.DeepCopy()
	// Store genesis block into db.
	gspec.MustCommit(db)
}

// AddNodeAddressesToGenesisAlloc adds to the genesis block allocation the accounts used for network validators/nodes,
// including the account used by the nodes of the initial beacon chain and later new nodes.
func AddNodeAddressesToGenesisAlloc(genesisAlloc core.GenesisAlloc) {
	for _, account := range genesis.IntelchainAccounts {
		testBankFunds := big.NewInt(core.InitFreeFund)
		testBankFunds = testBankFunds.Mul(testBankFunds, big.NewInt(denominations.Itc))
		address := common2.ParseAddr(account.Address)
		genesisAlloc[address] = core.GenesisAccount{Balance: testBankFunds}
	}
}
