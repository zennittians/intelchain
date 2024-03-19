package node

import (
	"fmt"

	msg_pb "github.com/zennittians/intelchain/api/proto/message"
	"github.com/zennittians/intelchain/api/service"
	"github.com/zennittians/intelchain/api/service/blockproposal"
	"github.com/zennittians/intelchain/api/service/consensus"
	"github.com/zennittians/intelchain/api/service/explorer"
	"github.com/zennittians/intelchain/api/service/networkinfo"
	nodeconfig "github.com/zennittians/intelchain/internal/configs/node"
	"github.com/zennittians/intelchain/internal/utils"
)

func (node *Node) setupForValidator() {
	_, chanPeer, _ := node.initNodeConfiguration()
	// Register networkinfo service. "0" is the beacon shard ID
	node.serviceManager.RegisterService(
		service.NetworkInfo,
		networkinfo.MustNew(
			node.host, node.NodeConfig.GetShardGroupID(), chanPeer, nil, node.networkInfoDHTPath(),
		),
	)
	// Register consensus service.
	node.serviceManager.RegisterService(
		service.Consensus,
		consensus.New(node.BlockChannel, node.Consensus, node.startConsensus),
	)
	// Register new block service.
	node.serviceManager.RegisterService(
		service.BlockProposal,
		blockproposal.New(node.Consensus.ReadySignal, node.Consensus.CommitSigChannel, node.WaitForConsensusReadyV2),
	)
}

func (node *Node) setupForExplorerNode() {
	_, chanPeer, _ := node.initNodeConfiguration()

	// Register networkinfo service.
	node.serviceManager.RegisterService(
		service.NetworkInfo,
		networkinfo.MustNew(
			node.host, node.NodeConfig.GetShardGroupID(), chanPeer, nil, node.networkInfoDHTPath()),
	)
	// Register explorer service.
	node.serviceManager.RegisterService(
		service.SupportExplorer, explorer.New(&node.SelfPeer, node.stateSync, node.Blockchain()),
	)
}

// ServiceManagerSetup setups service store.
func (node *Node) ServiceManagerSetup() {
	node.serviceManager = &service.Manager{}
	node.serviceMessageChan = make(map[service.Type]chan *msg_pb.Message)
	switch node.NodeConfig.Role() {
	case nodeconfig.Validator:
		node.setupForValidator()
	case nodeconfig.ExplorerNode:
		node.setupForExplorerNode()
	}
	node.serviceManager.SetupServiceMessageChan(node.serviceMessageChan)
}

// RunServices runs registered services.
func (node *Node) RunServices() {
	if node.serviceManager == nil {
		utils.Logger().Info().Msg("Service manager is not set up yet.")
		return
	}
	node.serviceManager.RunServices()
}

// StopServices runs registered services.
func (node *Node) StopServices() {
	if node.serviceManager == nil {
		utils.Logger().Info().Msg("Service manager is not set up yet.")
		return
	}
	node.serviceManager.StopServicesByRole([]service.Type{})
}

func (node *Node) networkInfoDHTPath() string {
	return fmt.Sprintf(".dht-%s-%s-c%s",
		node.SelfPeer.IP,
		node.SelfPeer.Port,
		node.chainConfig.ChainID,
	)
}
