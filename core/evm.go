// Copyright 2016 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package core

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/zennittians/intelchain/block"
	consensus_engine "github.com/zennittians/intelchain/consensus/engine"
	"github.com/zennittians/intelchain/core/types"
	"github.com/zennittians/intelchain/core/vm"
	staking "github.com/zennittians/intelchain/staking/types"
)

// ChainContext supports retrieving headers and consensus parameters from the
// current blockchain to be used during transaction processing.
type ChainContext interface {
	// Engine retrieves the chain's consensus engine.
	Engine() consensus_engine.Engine

	// GetHeader returns the hash corresponding to their hash.
	GetHeader(common.Hash, uint64) *block.Header

	// ReadDelegationsByDelegator returns the validators list of a delegator
	ReadDelegationsByDelegator(common.Address) (staking.DelegationIndexes, error)

	// ReadValidatorSnapshot returns the snapshot of validator at the beginning of current epoch.
	ReadValidatorSnapshot(common.Address) (*staking.ValidatorSnapshot, error)

	// ReadValidatorList returns the list of all validators
	ReadValidatorList() ([]common.Address, error)
}

// NewEVMContext creates a new context for use in the EVM.
func NewEVMContext(msg Message, header *block.Header, chain ChainContext, author *common.Address) vm.Context {
	// If we don't have an explicit author (i.e. not mining), extract from the header
	var beneficiary common.Address
	if author == nil {
		beneficiary = common.Address{} // Ignore error, we're past header validation
	} else {
		beneficiary = *author
	}
	return vm.Context{
		CanTransfer: CanTransfer,
		Transfer:    Transfer,
		IsValidator: IsValidator,
		GetHash:     GetHashFn(header, chain),
		Origin:      msg.From(),
		Coinbase:    beneficiary,
		BlockNumber: header.Number(),
		EpochNumber: header.Epoch(),
		Time:        header.Time(),
		GasLimit:    header.GasLimit(),
		GasPrice:    new(big.Int).Set(msg.GasPrice()),
	}
}

// GetHashFn returns a GetHashFunc which retrieves header hashes by number
func GetHashFn(ref *block.Header, chain ChainContext) func(n uint64) common.Hash {
	var cache map[uint64]common.Hash

	return func(n uint64) common.Hash {
		// If there's no hash cache yet, make one
		if cache == nil {
			cache = map[uint64]common.Hash{
				ref.Number().Uint64() - 1: ref.ParentHash(),
			}
		}
		// Try to fulfill the request from the cache
		if hash, ok := cache[n]; ok {
			return hash
		}
		// Not cached, iterate the blocks and cache the hashes
		for header := chain.GetHeader(ref.ParentHash(), ref.Number().Uint64()-1); header != nil; header = chain.GetHeader(header.ParentHash(), header.Number().Uint64()-1) {
			cache[header.Number().Uint64()-1] = header.ParentHash()
			if n == header.Number().Uint64()-1 {
				return header.ParentHash()
			}
		}
		return common.Hash{}
	}
}

// CanTransfer checks whether there are enough funds in the address' account to make a transfer.
// This does not take the necessary gas in to account to make the transfer valid.
func CanTransfer(db vm.StateDB, addr common.Address, amount *big.Int) bool {
	return db.GetBalance(addr).Cmp(amount) >= 0
}

// IsValidator determines whether it is a validator address or not
func IsValidator(db vm.StateDB, addr common.Address) bool {
	return db.IsValidator(addr)
}

// Transfer subtracts amount from sender and adds amount to recipient using the given Db
func Transfer(db vm.StateDB, sender, recipient common.Address, amount *big.Int, txType types.TransactionType) {
	if txType == types.SameShardTx || txType == types.SubtractionOnly {
		db.SubBalance(sender, amount)
	}
	if txType == types.SameShardTx {
		db.AddBalance(recipient, amount)
	}
}
