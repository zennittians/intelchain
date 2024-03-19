package rpc

import (
	"context"

	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/zennittians/intelchain/internal/utils"
	"github.com/zennittians/intelchain/itc"
)

// PrivateDebugService Internal JSON RPC for debugging purpose
type PrivateDebugService struct {
	itc     *itc.Intelchain
	version Version
}

// NewPrivateDebugAPI creates a new API for the RPC interface
// TODO(dm): expose public via config
func NewPrivateDebugAPI(itc *itc.Intelchain, version Version) rpc.API {
	return rpc.API{
		Namespace: version.Namespace(),
		Version:   APIVersion,
		Service:   &PrivateDebugService{itc, version},
		Public:    false,
	}
}

// SetLogVerbosity Sets log verbosity on runtime
func (s *PrivateDebugService) SetLogVerbosity(ctx context.Context, level int) (map[string]interface{}, error) {
	if level < int(log.LvlCrit) || level > int(log.LvlTrace) {
		return nil, ErrInvalidLogLevel
	}

	verbosity := log.Lvl(level)
	utils.SetLogVerbosity(verbosity)
	return map[string]interface{}{"verbosity": verbosity.String()}, nil
}

// ConsensusViewChangingID return the current view changing ID to RPC
func (s *PrivateDebugService) ConsensusViewChangingID(
	ctx context.Context,
) uint64 {
	return s.itc.NodeAPI.GetConsensusViewChangingID()
}

// ConsensusCurViewID return the current view ID to RPC
func (s *PrivateDebugService) ConsensusCurViewID(
	ctx context.Context,
) uint64 {
	return s.itc.NodeAPI.GetConsensusCurViewID()
}

// GetConsensusMode return the current consensus mode
func (s *PrivateDebugService) GetConsensusMode(
	ctx context.Context,
) string {
	return s.itc.NodeAPI.GetConsensusMode()
}

// GetConsensusPhase return the current consensus mode
func (s *PrivateDebugService) GetConsensusPhase(
	ctx context.Context,
) string {
	return s.itc.NodeAPI.GetConsensusPhase()
}
