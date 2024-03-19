package v1

import (
	"context"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/rpc"
	internal_common "github.com/zennittians/intelchain/internal/common"
	"github.com/zennittians/intelchain/itc"
)

// PublicEthService provides an API to access to the Eth endpoints for the Intelchain blockchain.
type PublicEthService struct {
	itc *itc.Intelchain
}

// NewPublicEthService creates a new API for the RPC interface
func NewPublicEthService(itc *itc.Intelchain, namespace string) rpc.API {
	if namespace == "" {
		namespace = "eth"
	}

	return rpc.API{
		Namespace: namespace,
		Version:   "1.0",
		Service:   &PublicEthService{itc},
		Public:    true,
	}
}

// GetBalance returns the amount of Atto for the given address in the state of the
// given block number. The rpc.LatestBlockNumber and rpc.PendingBlockNumber meta
// block numbers are also allowed.
func (s *PublicEthService) GetBalance(
	ctx context.Context, address string, blockNr rpc.BlockNumber,
) (*hexutil.Big, error) {
	addr := internal_common.ParseAddr(address)
	balance, err := s.itc.GetBalance(ctx, addr, blockNr)
	if err != nil {
		return nil, err
	}
	return (*hexutil.Big)(balance), nil
}
