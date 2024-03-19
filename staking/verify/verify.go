package verify

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/pkg/errors"
	"github.com/zennittians/bls/ffi/go/bls"
	"github.com/zennittians/intelchain/consensus/quorum"
	"github.com/zennittians/intelchain/consensus/signature"
	"github.com/zennittians/intelchain/core"
	bls_cosi "github.com/zennittians/intelchain/crypto/bls"
	"github.com/zennittians/intelchain/shard"
)

var (
	errQuorumVerifyAggSign = errors.New("insufficient voting power to verify aggregate sig")
	errAggregateSigFail    = errors.New("could not verify hash of aggregate signature")
)

// AggregateSigForCommittee ..
func AggregateSigForCommittee(
	chain *core.BlockChain,
	committee *shard.Committee,
	decider quorum.Decider,
	aggSignature *bls.Sign,
	hash common.Hash,
	blockNum, viewID uint64,
	epoch *big.Int,
	bitmap []byte,
) error {
	committerKeys, err := committee.BLSPublicKeys()
	if err != nil {
		return err
	}
	mask, err := bls_cosi.NewMask(committerKeys, nil)
	if err != nil {
		return err
	}
	if err := mask.SetMask(bitmap); err != nil {
		return err
	}

	if !decider.IsQuorumAchievedByMask(mask) {
		return errQuorumVerifyAggSign
	}

	commitPayload := signature.ConstructCommitPayload(chain, epoch, hash, blockNum, viewID)
	if !aggSignature.VerifyHash(mask.AggregatePublic, commitPayload) {
		return errAggregateSigFail
	}

	return nil
}
