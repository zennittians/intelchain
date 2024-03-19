package consensus

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/zennittians/abool"
	"github.com/zennittians/intelchain/consensus/quorum"
	"github.com/zennittians/intelchain/crypto/bls"
	"github.com/zennittians/intelchain/internal/utils"
	"github.com/zennittians/intelchain/multibls"
	"github.com/zennittians/intelchain/p2p"
	"github.com/zennittians/intelchain/shard"
	"github.com/zennittians/intelchain/staking/slash"
	"github.com/zennittians/intelchain/test/helpers"
)

func TestConsensusInitialization(t *testing.T) {
	host, multiBLSPrivateKey, consensus, decider, err := GenerateConsensusForTesting()
	assert.NoError(t, err)

	messageSender := &MessageSender{host: host, retryTimes: int(phaseDuration.Seconds()) / RetryIntervalInSec}
	fbtLog := NewFBFTLog()
	state := State{mode: Normal}

	timeouts := createTimeout()
	expectedTimeouts := make(map[TimeoutType]time.Duration)
	expectedTimeouts[timeoutConsensus] = phaseDuration
	expectedTimeouts[timeoutViewChange] = viewChangeDuration
	expectedTimeouts[timeoutBootstrap] = bootstrapDuration

	assert.Equal(t, decider, consensus.Decider)
	assert.Equal(t, host, consensus.host)
	assert.Equal(t, messageSender, consensus.msgSender)
	assert.IsType(t, make(chan struct{}), consensus.BlockNumLowChan)

	// FBFTLog
	assert.Equal(t, fbtLog, consensus.FBFTLog)

	assert.Equal(t, FBFTAnnounce, consensus.phase)

	// State / consensus.current
	assert.Equal(t, state.mode, consensus.current.mode)
	assert.Equal(t, state.GetViewChangingID(), consensus.current.GetViewChangingID())

	// FBFT timeout
	assert.IsType(t, make(map[TimeoutType]*utils.Timeout), consensus.consensusTimeout)
	for timeoutType, timeout := range expectedTimeouts {
		duration := consensus.consensusTimeout[timeoutType].Duration()
		assert.Equal(t, timeouts[timeoutType].Duration().Nanoseconds(), duration.Nanoseconds())
		assert.Equal(t, timeout.Nanoseconds(), duration.Nanoseconds())
	}

	// MultiBLS
	assert.Equal(t, multiBLSPrivateKey, consensus.priKey)
	assert.Equal(t, multiBLSPrivateKey.GetPublicKeys(), consensus.GetPublicKeys())

	// Misc
	assert.Equal(t, uint64(0), consensus.GetViewChangingID())
	assert.Equal(t, uint32(shard.BeaconChainShardID), consensus.ShardID)

	assert.IsType(t, make(chan struct{}), consensus.syncReadyChan)
	assert.NotNil(t, consensus.syncReadyChan)

	assert.IsType(t, make(chan struct{}), consensus.syncNotReadyChan)
	assert.NotNil(t, consensus.syncNotReadyChan)

	assert.IsType(t, make(chan slash.Record), consensus.SlashChan)
	assert.NotNil(t, consensus.SlashChan)

	assert.IsType(t, make(chan ProposalType), consensus.ReadySignal)
	assert.NotNil(t, consensus.ReadySignal)

	assert.IsType(t, make(chan [vdfAndSeedSize]byte), consensus.RndChannel)
	assert.NotNil(t, consensus.RndChannel)

	assert.IsType(t, abool.NewBool(false), consensus.IgnoreViewIDCheck)
	assert.NotNil(t, consensus.IgnoreViewIDCheck)
}

// GenerateConsensusForTesting - helper method to generate a basic consensus
func GenerateConsensusForTesting() (p2p.Host, multibls.PrivateKeys, *Consensus, quorum.Decider, error) {
	hostData := helpers.Hosts[0]
	host, _, err := helpers.GenerateHost(hostData.IP, hostData.Port)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	peer := host.GetSelfPeer()

	decider := quorum.NewDecider(quorum.SuperMajorityVote, shard.BeaconChainShardID)
	multiBLSPrivateKey := multibls.GetPrivateKeys(bls.RandPrivateKey())

	consensus, err := New(host, shard.BeaconChainShardID, peer, multiBLSPrivateKey, decider)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	return host, multiBLSPrivateKey, consensus, decider, nil
}
