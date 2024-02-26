package rpc

import (
	"context"

	"github.com/zennittians/intelchain/core"
	"github.com/zennittians/intelchain/eth/rpc"
	"github.com/zennittians/intelchain/hmy"
)

type PreimagesService struct {
	hmy *hmy.Harmony
}

// NewPreimagesAPI creates a new API for the RPC interface
func NewPreimagesAPI(hmy *hmy.Harmony, version string) rpc.API {
	var service interface{} = &PreimagesService{hmy}
	return rpc.API{
		Namespace: version,
		Version:   APIVersion,
		Service:   service,
		Public:    true,
	}
}

func (s *PreimagesService) Export(ctx context.Context, path string) error {
	// these are by default not blocking
	return core.ExportPreimages(s.hmy.BlockChain, path)
}

func (s *PreimagesService) Verify(ctx context.Context) (uint64, error) {
	currentBlock := s.hmy.CurrentBlock()
	// these are by default not blocking
	return core.VerifyPreimages(currentBlock.Header(), s.hmy.BlockChain)
}
