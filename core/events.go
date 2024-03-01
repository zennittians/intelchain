// Copyright 2014 The go-ethereum Authors
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
	"github.com/ethereum/go-ethereum/common"
	"github.com/zennittians/intelchain/core/types"
	"github.com/zennittians/intelchain/itc/tracers"
)

// NewTxsEvent is posted when a batch of transactions enter the transaction pool.
type NewTxsEvent struct{ Txs []types.PoolTransaction }

// PendingLogsEvent is posted pre mining and notifies of pending logs.
type PendingLogsEvent struct {
	Logs []*types.Log
}

// NewMinedBlockEvent is posted when a block has been imported.
type NewMinedBlockEvent struct{ Block *types.Block }

// RemovedLogsEvent is posted when a reorg happens
type RemovedLogsEvent struct{ Logs []*types.Log }

// ChainEvent is the struct of chain event.
type ChainEvent struct {
	Block *types.Block
	Hash  common.Hash
	Logs  []*types.Log
}

type TraceEvent struct {
	Tracer *tracers.ParityBlockTracer
}

// ChainSideEvent is chain side event.
type ChainSideEvent struct {
	Block *types.Block
}

// ChainHeadEvent is the struct of chain head event.
type ChainHeadEvent struct{ Block *types.Block }
