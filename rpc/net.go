package rpc

import (
	"context"
	"fmt"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/rpc"
	nodeconfig "github.com/zennittians/intelchain/internal/configs/node"
	"github.com/zennittians/intelchain/internal/utils"
	"github.com/zennittians/intelchain/p2p"
)

// PublicNetService offers network related RPC methods
type PublicNetService struct {
	net     p2p.Host
	chainID uint64
	version Version
}

// NewPublicNetAPI creates a new net API instance.
func NewPublicNetAPI(net p2p.Host, chainID uint64, version Version) rpc.API {
	// manually set different namespace to preserve legacy behavior
	var namespace string
	switch version {
	case V1:
		namespace = netV1Namespace
	case V2:
		namespace = netV2Namespace
	case Eth:
		namespace = netNamespace
	default:
		utils.Logger().Error().Msgf("Unknown version %v, ignoring API.", version)
		return rpc.API{}
	}

	return rpc.API{
		Namespace: namespace,
		Version:   APIVersion,
		Service:   &PublicNetService{net, chainID, version},
		Public:    true,
	}
}

// PeerCount returns the number of connected peers
// Note that the return type is an interface to account for the different versions
func (s *PublicNetService) PeerCount(ctx context.Context) (interface{}, error) {
	// Format response according to version
	switch s.version {
	case V1, Eth:
		return hexutil.Uint(s.net.GetPeerCount()), nil
	case V2:
		return s.net.GetPeerCount(), nil
	default:
		return nil, ErrUnknownRPCVersion
	}
}

// Version returns the network version, i.e. ChainID identifying which network we are using
func (s *PublicNetService) Version(ctx context.Context) interface{} {
	switch s.version {
	case Eth:
		return hexutil.Uint64(nodeconfig.GetDefaultConfig().GetNetworkType().ChainConfig().EthCompatibleChainID.Uint64())
	default:
		return fmt.Sprintf("%d", s.chainID)
	}
}
