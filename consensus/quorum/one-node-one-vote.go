package quorum

import (
	"encoding/json"
	"math/big"

	bls_core "github.com/zennittians/bls/ffi/go/bls"
	"github.com/zennittians/intelchain/crypto/bls"

	"github.com/ethereum/go-ethereum/common"
	"github.com/zennittians/intelchain/consensus/votepower"

	bls_cosi "github.com/zennittians/intelchain/crypto/bls"
	"github.com/zennittians/intelchain/internal/utils"
	"github.com/zennittians/intelchain/numeric"
	"github.com/zennittians/intelchain/shard"
)

type uniformVoteWeight struct {
	DependencyInjectionWriter
	DependencyInjectionReader
	SignatureReader
}

// Policy ..
func (v *uniformVoteWeight) Policy() Policy {
	return SuperMajorityVote
}

// AddNewVote ..
func (v *uniformVoteWeight) AddNewVote(
	p Phase, pubKeys []*bls_cosi.PublicKeyWrapper,
	sig *bls_core.Sign, headerHash common.Hash,
	height, viewID uint64) (*votepower.Ballot, error) {
	pubKeysBytes := make([]bls.SerializedPublicKey, len(pubKeys))
	for i, pubKey := range pubKeys {
		pubKeysBytes[i] = pubKey.Bytes
	}
	return v.submitVote(p, pubKeysBytes, sig, headerHash, height, viewID)
}

// IsQuorumAchieved ..
func (v *uniformVoteWeight) IsQuorumAchieved(p Phase) bool {
	r := v.SignersCount(p) >= v.TwoThirdsSignersCount()
	utils.Logger().Info().Str("phase", p.String()).
		Int64("signers-count", v.SignersCount(p)).
		Int64("threshold", v.TwoThirdsSignersCount()).
		Int64("participants", v.ParticipantsCount()).
		Msg("Quorum details")
	return r
}

// IsQuorumAchivedByMask ..
func (v *uniformVoteWeight) IsQuorumAchievedByMask(mask *bls_cosi.Mask) bool {
	if mask == nil {
		return false
	}
	threshold := v.TwoThirdsSignersCount()
	currentTotalPower := utils.CountOneBits(mask.Bitmap)
	if currentTotalPower < threshold {
		const msg = "[IsQuorumAchievedByMask] Not enough voting power: need %+v, have %+v"
		utils.Logger().Warn().Msgf(msg, threshold, currentTotalPower)
		return false
	}
	const msg = "[IsQuorumAchievedByMask] have enough voting power: need %+v, have %+v"
	utils.Logger().Debug().
		Msgf(msg, threshold, currentTotalPower)
	return true
}

// QuorumThreshold ..
func (v *uniformVoteWeight) QuorumThreshold() numeric.Dec {
	return numeric.NewDec(v.TwoThirdsSignersCount())
}

// IsAllSigsCollected ..
func (v *uniformVoteWeight) IsAllSigsCollected() bool {
	return v.SignersCount(Commit) == v.ParticipantsCount()
}

func (v *uniformVoteWeight) SetVoters(
	subCommittee *shard.Committee, epoch *big.Int,
) (*TallyResult, error) {
	// NO-OP do not add anything here
	return nil, nil
}

func (v *uniformVoteWeight) String() string {
	s, _ := json.Marshal(v)
	return string(s)
}

func (v *uniformVoteWeight) MarshalJSON() ([]byte, error) {
	type t struct {
		Policy       string   `json:"policy"`
		Count        int      `json:"count"`
		Participants []string `json:"committee-members"`
	}
	keysDump := v.Participants()
	keys := make([]string, len(keysDump))
	for i := range keysDump {
		keys[i] = keysDump[i].Bytes.Hex()
	}

	return json.Marshal(t{v.Policy().String(), len(keys), keys})
}

func (v *uniformVoteWeight) AmIMemberOfCommitee() bool {
	pubKeyFunc := v.MyPublicKey()
	if pubKeyFunc == nil {
		return false
	}
	identity, _ := pubKeyFunc()
	everyone := v.Participants()
	for _, key := range identity {
		for i := range everyone {
			if key.Object.IsEqual(everyone[i].Object) {
				return true
			}
		}
	}
	return false
}

func (v *uniformVoteWeight) ResetPrepareAndCommitVotes() {
	v.reset([]Phase{Prepare, Commit})
}

func (v *uniformVoteWeight) ResetViewChangeVotes() {
	v.reset([]Phase{ViewChange})
}
