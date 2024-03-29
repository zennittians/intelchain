package rpc

import (
	"context"
	"fmt"
	"math/big"
	"strings"

	"github.com/pkg/errors"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/zennittians/intelchain/core/rawdb"
	"github.com/zennittians/intelchain/core/types"
	"github.com/zennittians/intelchain/core/vm"
	internal_common "github.com/zennittians/intelchain/internal/common"
	"github.com/zennittians/intelchain/internal/params"
	"github.com/zennittians/intelchain/internal/utils"
	"github.com/zennittians/intelchain/itc"
	eth "github.com/zennittians/intelchain/rpc/eth"
	v1 "github.com/zennittians/intelchain/rpc/v1"
	v2 "github.com/zennittians/intelchain/rpc/v2"
	staking "github.com/zennittians/intelchain/staking/types"
)

const (
	defaultPageSize = uint32(100)
)

// PublicTransactionService provides an API to access Intelchain's transaction service.
// It offers only methods that operate on public data that is freely available to anyone.
type PublicTransactionService struct {
	itc     *itc.Intelchain
	version Version
}

// NewPublicTransactionAPI creates a new API for the RPC interface
func NewPublicTransactionAPI(itc *itc.Intelchain, version Version) rpc.API {
	return rpc.API{
		Namespace: version.Namespace(),
		Version:   APIVersion,
		Service:   &PublicTransactionService{itc, version},
		Public:    true,
	}
}

// GetAccountNonce returns the nonce value of the given address for the given block number
func (s *PublicTransactionService) GetAccountNonce(
	ctx context.Context, address string, blockNumber BlockNumber,
) (uint64, error) {
	// Process number based on version
	blockNum := blockNumber.EthBlockNumber()

	// Response output is the same for all versions
	addr := internal_common.ParseAddr(address)
	return s.itc.GetAccountNonce(ctx, addr, blockNum)
}

// GetTransactionCount returns the number of transactions the given address has sent for the given block number.
// Legacy for apiv1. For apiv2, please use getAccountNonce/getPoolNonce/getTransactionsCount/getStakingTransactionsCount apis for
// more granular transaction counts queries
// Note that the return type is an interface to account for the different versions
func (s *PublicTransactionService) GetTransactionCount(
	ctx context.Context, addr string, blockNumber BlockNumber,
) (response interface{}, err error) {
	// Process arguments based on version
	blockNum := blockNumber.EthBlockNumber()
	address := internal_common.ParseAddr(addr)

	// Fetch transaction count
	var nonce uint64
	if blockNum == rpc.PendingBlockNumber {
		// Ask transaction pool for the nonce which includes pending transactions
		nonce, err = s.itc.GetPoolNonce(ctx, address)
		if err != nil {
			return nil, err
		}
	} else {
		// Resolve block number and use its state to ask for the nonce
		state, _, err := s.itc.StateAndHeaderByNumber(ctx, blockNum)
		if err != nil {
			return nil, err
		}
		if state == nil {
			return nil, fmt.Errorf("state not found")
		}
		if state.Error() != nil {
			return nil, state.Error()
		}
		nonce = state.GetNonce(address)
	}

	// Format response according to version
	switch s.version {
	case V1, Eth:
		return (hexutil.Uint64)(nonce), nil
	case V2:
		return nonce, nil
	default:
		return nil, ErrUnknownRPCVersion
	}
}

// GetTransactionsCount returns the number of regular transactions from genesis of input type ("SENT", "RECEIVED", "ALL")
func (s *PublicTransactionService) GetTransactionsCount(
	ctx context.Context, address, txType string,
) (count uint64, err error) {
	if !strings.HasPrefix(address, "one1") {
		// Handle hex address
		addr := internal_common.ParseAddr(address)
		address, err = internal_common.AddressToBech32(addr)
		if err != nil {
			return 0, err
		}
	}

	// Response output is the same for all versions
	return s.itc.GetTransactionsCount(address, txType)
}

// GetStakingTransactionsCount returns the number of staking transactions from genesis of input type ("SENT", "RECEIVED", "ALL")
func (s *PublicTransactionService) GetStakingTransactionsCount(
	ctx context.Context, address, txType string,
) (count uint64, err error) {
	if !strings.HasPrefix(address, "one1") {
		// Handle hex address
		addr := internal_common.ParseAddr(address)
		address, err = internal_common.AddressToBech32(addr)
		if err != nil {
			return 0, err
		}
	}

	// Response output is the same for all versions
	return s.itc.GetStakingTransactionsCount(address, txType)
}

// EstimateGas returns an estimate of the amount of gas needed to execute the
// given transaction against the current pending block.
func (s *PublicTransactionService) EstimateGas(
	ctx context.Context, args CallArgs,
) (hexutil.Uint64, error) {
	gas, err := EstimateGas(ctx, s.itc, args, nil)
	if err != nil {
		return 0, err
	}

	// Response output is the same for all versions
	return (hexutil.Uint64)(gas), nil
}

// GetTransactionByHash returns the plain transaction for the given hash
func (s *PublicTransactionService) GetTransactionByHash(
	ctx context.Context, hash common.Hash,
) (StructuredResponse, error) {
	// Try to return an already finalized transaction
	tx, blockHash, blockNumber, index := rawdb.ReadTransaction(s.itc.ChainDb(), hash)
	if tx == nil {
		utils.Logger().Debug().
			Err(errors.Wrapf(ErrTransactionNotFound, "hash %v", hash.String())).
			Msgf("%v error at %v", LogTag, "GetTransactionByHash")
		// Legacy behavior is to not return RPC errors
		return nil, nil
	}
	block, err := s.itc.GetBlock(ctx, blockHash)
	if err != nil {
		utils.Logger().Debug().
			Err(err).
			Msgf("%v error at %v", LogTag, "GetTransactionByHash")
		// Legacy behavior is to not return RPC errors
		return nil, nil
	}

	// Format the response according to the version
	switch s.version {
	case V1:
		tx, err := v1.NewTransaction(tx, blockHash, blockNumber, block.Time().Uint64(), index)
		if err != nil {
			return nil, err
		}
		return NewStructuredResponse(tx)
	case V2:
		tx, err := v2.NewTransaction(tx, blockHash, blockNumber, block.Time().Uint64(), index)
		if err != nil {
			return nil, err
		}
		return NewStructuredResponse(tx)
	case Eth:
		tx, err := eth.NewTransaction(tx, blockHash, blockNumber, block.Time().Uint64(), index)
		if err != nil {
			return nil, err
		}
		return NewStructuredResponse(tx)
	default:
		return nil, ErrUnknownRPCVersion
	}
}

// GetStakingTransactionByHash returns the staking transaction for the given hash
func (s *PublicTransactionService) GetStakingTransactionByHash(
	ctx context.Context, hash common.Hash,
) (StructuredResponse, error) {
	// Try to return an already finalized transaction
	stx, blockHash, blockNumber, index := rawdb.ReadStakingTransaction(s.itc.ChainDb(), hash)
	if stx == nil {
		utils.Logger().Debug().
			Err(errors.Wrapf(ErrTransactionNotFound, "hash %v", hash.String())).
			Msgf("%v error at %v", LogTag, "GetStakingTransactionByHash")
		// Legacy behavior is to not return RPC errors
		return nil, nil
	}
	block, err := s.itc.GetBlock(ctx, blockHash)
	if err != nil {
		utils.Logger().Debug().
			Err(err).
			Msgf("%v error at %v", LogTag, "GetStakingTransactionByHash")
		// Legacy behavior is to not return RPC errors
		return nil, nil
	}

	switch s.version {
	case V1:
		tx, err := v1.NewStakingTransaction(stx, blockHash, blockNumber, block.Time().Uint64(), index)
		if err != nil {
			return nil, err
		}
		return NewStructuredResponse(tx)
	case V2:
		tx, err := v2.NewStakingTransaction(stx, blockHash, blockNumber, block.Time().Uint64(), index)
		if err != nil {
			return nil, err
		}
		return NewStructuredResponse(tx)
	default:
		return nil, ErrUnknownRPCVersion
	}
}

// GetTransactionsHistory returns the list of transactions hashes that involve a particular address.
func (s *PublicTransactionService) GetTransactionsHistory(
	ctx context.Context, args TxHistoryArgs,
) (StructuredResponse, error) {
	// Fetch transaction history
	var address string
	var result []common.Hash
	var err error
	if strings.HasPrefix(args.Address, "one1") {
		address = args.Address
	} else {
		addr := internal_common.ParseAddr(args.Address)
		address, err = internal_common.AddressToBech32(addr)
		if err != nil {
			return nil, err
		}
	}
	hashes, err := s.itc.GetTransactionsHistory(address, args.TxType, args.Order)
	if err != nil {
		return nil, err
	}

	result = returnHashesWithPagination(hashes, args.PageIndex, args.PageSize)

	// Just hashes have same response format for all versions
	if !args.FullTx {
		return StructuredResponse{"transactions": result}, nil
	}

	// Full transactions have different response format
	txs := []StructuredResponse{}
	for _, hash := range result {
		tx, err := s.GetTransactionByHash(ctx, hash)
		if err == nil {
			txs = append(txs, tx)
		} else {
			utils.Logger().Debug().
				Err(err).
				Msgf("%v error at %v", LogTag, "GetTransactionsHistory")
			// Legacy behavior is to not return RPC errors
		}
	}
	return StructuredResponse{"transactions": txs}, nil
}

// GetStakingTransactionsHistory returns the list of transactions hashes that involve a particular address.
func (s *PublicTransactionService) GetStakingTransactionsHistory(
	ctx context.Context, args TxHistoryArgs,
) (StructuredResponse, error) {
	// Fetch transaction history
	var address string
	var result []common.Hash
	var err error
	if strings.HasPrefix(args.Address, "one1") {
		address = args.Address
	} else {
		addr := internal_common.ParseAddr(args.Address)
		address, err = internal_common.AddressToBech32(addr)
		if err != nil {
			utils.Logger().Debug().
				Err(err).
				Msgf("%v error at %v", LogTag, "GetStakingTransactionsHistory")
			// Legacy behavior is to not return RPC errors
			return nil, nil
		}
	}
	hashes, err := s.itc.GetStakingTransactionsHistory(address, args.TxType, args.Order)
	if err != nil {
		utils.Logger().Debug().
			Err(err).
			Msgf("%v error at %v", LogTag, "GetStakingTransactionsHistory")
		// Legacy behavior is to not return RPC errors
		return nil, nil
	}

	result = returnHashesWithPagination(hashes, args.PageIndex, args.PageSize)

	// Just hashes have same response format for all versions
	if !args.FullTx {
		return StructuredResponse{"staking_transactions": result}, nil
	}

	// Full transactions have different response format
	txs := []StructuredResponse{}
	for _, hash := range result {
		tx, err := s.GetStakingTransactionByHash(ctx, hash)
		if err == nil {
			txs = append(txs, tx)
		} else {
			utils.Logger().Debug().
				Err(err).
				Msgf("%v error at %v", LogTag, "GetStakingTransactionsHistory")
			// Legacy behavior is to not return RPC errors
		}
	}
	return StructuredResponse{"staking_transactions": txs}, nil
}

// GetBlockTransactionCountByNumber returns the number of transactions in the block with the given block number.
// Note that the return type is an interface to account for the different versions
func (s *PublicTransactionService) GetBlockTransactionCountByNumber(
	ctx context.Context, blockNumber BlockNumber,
) (interface{}, error) {
	// Process arguments based on version
	blockNum := blockNumber.EthBlockNumber()

	// Fetch block
	block, err := s.itc.BlockByNumber(ctx, blockNum)
	if err != nil {
		utils.Logger().Debug().
			Err(err).
			Msgf("%v error at %v", LogTag, "GetBlockTransactionCountByNumber")
		// Legacy behavior is to not return RPC errors
		return nil, nil
	}

	// Format response according to version
	switch s.version {
	case V1, Eth:
		return hexutil.Uint(len(block.Transactions())), nil
	case V2:
		return len(block.Transactions()), nil
	default:
		return nil, ErrUnknownRPCVersion
	}
}

// GetBlockTransactionCountByHash returns the number of transactions in the block with the given hash.
// Note that the return type is an interface to account for the different versions
func (s *PublicTransactionService) GetBlockTransactionCountByHash(
	ctx context.Context, blockHash common.Hash,
) (interface{}, error) {
	// Fetch block
	block, err := s.itc.GetBlock(ctx, blockHash)
	if err != nil {
		utils.Logger().Debug().
			Err(err).
			Msgf("%v error at %v", LogTag, "GetBlockTransactionCountByHash")
		// Legacy behavior is to not return RPC errors
		return nil, nil
	}

	// Format response according to version
	switch s.version {
	case V1, Eth:
		return hexutil.Uint(len(block.Transactions())), nil
	case V2:
		return len(block.Transactions()), nil
	default:
		return nil, ErrUnknownRPCVersion
	}
}

// GetTransactionByBlockNumberAndIndex returns the transaction for the given block number and index.
func (s *PublicTransactionService) GetTransactionByBlockNumberAndIndex(
	ctx context.Context, blockNumber BlockNumber, index TransactionIndex,
) (StructuredResponse, error) {
	// Process arguments based on version
	blockNum := blockNumber.EthBlockNumber()

	// Fetch Block
	block, err := s.itc.BlockByNumber(ctx, blockNum)
	if err != nil {
		utils.Logger().Debug().
			Err(err).
			Msgf("%v error at %v", LogTag, "GetTransactionByBlockNumberAndIndex")
		// Legacy behavior is to not return RPC errors
		return nil, nil
	}

	// Format response according to version
	switch s.version {
	case V1:
		tx, err := v1.NewTransactionFromBlockIndex(block, uint64(index))
		if err != nil {
			return nil, err
		}
		return NewStructuredResponse(tx)
	case V2:
		tx, err := v2.NewTransactionFromBlockIndex(block, uint64(index))
		if err != nil {
			return nil, err
		}
		return NewStructuredResponse(tx)
	case Eth:
		tx, err := eth.NewTransactionFromBlockIndex(block, uint64(index))
		if err != nil {
			return nil, err
		}
		return NewStructuredResponse(tx)
	default:
		return nil, ErrUnknownRPCVersion
	}
}

// GetTransactionByBlockHashAndIndex returns the transaction for the given block hash and index.
func (s *PublicTransactionService) GetTransactionByBlockHashAndIndex(
	ctx context.Context, blockHash common.Hash, index TransactionIndex,
) (StructuredResponse, error) {
	// Fetch Block
	block, err := s.itc.GetBlock(ctx, blockHash)
	if err != nil {
		utils.Logger().Debug().
			Err(err).
			Msgf("%v error at %v", LogTag, "GetTransactionByBlockHashAndIndex")
		// Legacy behavior is to not return RPC errors
		return nil, nil
	}

	// Format response according to version
	switch s.version {
	case V1:
		tx, err := v1.NewTransactionFromBlockIndex(block, uint64(index))
		if err != nil {
			return nil, err
		}
		return NewStructuredResponse(tx)
	case V2:
		tx, err := v2.NewTransactionFromBlockIndex(block, uint64(index))
		if err != nil {
			return nil, err
		}
		return NewStructuredResponse(tx)
	case Eth:
		tx, err := eth.NewTransactionFromBlockIndex(block, uint64(index))
		if err != nil {
			return nil, err
		}
		return NewStructuredResponse(tx)
	default:
		return nil, ErrUnknownRPCVersion
	}
}

// GetBlockStakingTransactionCountByNumber returns the number of staking transactions in the block with the given block number.
// Note that the return type is an interface to account for the different versions
func (s *PublicTransactionService) GetBlockStakingTransactionCountByNumber(
	ctx context.Context, blockNumber BlockNumber,
) (interface{}, error) {
	// Process arguments based on version
	blockNum := blockNumber.EthBlockNumber()

	// Fetch block
	block, err := s.itc.BlockByNumber(ctx, blockNum)
	if err != nil {
		utils.Logger().Debug().
			Err(err).
			Msgf("%v error at %v", LogTag, "GetBlockStakingTransactionCountByNumber")
		// Legacy behavior is to not return RPC errors
		return nil, nil
	}

	// Format response according to version
	switch s.version {
	case V1, Eth:
		return hexutil.Uint(len(block.StakingTransactions())), nil
	case V2:
		return len(block.StakingTransactions()), nil
	default:
		return nil, ErrUnknownRPCVersion
	}
}

// GetBlockStakingTransactionCountByHash returns the number of staking transactions in the block with the given hash.
// Note that the return type is an interface to account for the different versions
func (s *PublicTransactionService) GetBlockStakingTransactionCountByHash(
	ctx context.Context, blockHash common.Hash,
) (interface{}, error) {
	// Fetch block
	block, err := s.itc.GetBlock(ctx, blockHash)
	if err != nil {
		utils.Logger().Debug().
			Err(err).
			Msgf("%v error at %v", LogTag, "GetBlockStakingTransactionCountByHash")
		// Legacy behavior is to not return RPC errors
		return nil, nil
	}

	// Format response according to version
	switch s.version {
	case V1, Eth:
		return hexutil.Uint(len(block.StakingTransactions())), nil
	case V2:
		return len(block.StakingTransactions()), nil
	default:
		return nil, ErrUnknownRPCVersion
	}
}

// GetStakingTransactionByBlockNumberAndIndex returns the staking transaction for the given block number and index.
func (s *PublicTransactionService) GetStakingTransactionByBlockNumberAndIndex(
	ctx context.Context, blockNumber BlockNumber, index TransactionIndex,
) (StructuredResponse, error) {
	// Process arguments based on version
	blockNum := blockNumber.EthBlockNumber()

	// Fetch Block
	block, err := s.itc.BlockByNumber(ctx, blockNum)
	if err != nil {
		utils.Logger().Debug().
			Err(err).
			Msgf("%v error at %v", LogTag, "GetStakingTransactionByBlockNumberAndIndex")
		// Legacy behavior is to not return RPC errors
		return nil, nil
	}

	// Format response according to version
	switch s.version {
	case V1:
		tx, err := v1.NewStakingTransactionFromBlockIndex(block, uint64(index))
		if err != nil {
			return nil, err
		}
		return NewStructuredResponse(tx)
	case V2:
		tx, err := v2.NewStakingTransactionFromBlockIndex(block, uint64(index))
		if err != nil {
			return nil, err
		}
		return NewStructuredResponse(tx)
	default:
		return nil, ErrUnknownRPCVersion
	}
}

// GetStakingTransactionByBlockHashAndIndex returns the transaction for the given block hash and index.
func (s *PublicTransactionService) GetStakingTransactionByBlockHashAndIndex(
	ctx context.Context, blockHash common.Hash, index TransactionIndex,
) (StructuredResponse, error) {
	// Fetch Block
	block, err := s.itc.GetBlock(ctx, blockHash)
	if err != nil {
		utils.Logger().Debug().
			Err(err).
			Msgf("%v error at %v", LogTag, "GetStakingTransactionByBlockHashAndIndex")
		// Legacy behavior is to not return RPC errors
		return nil, nil
	}

	// Format response according to version
	switch s.version {
	case V1, Eth:
		tx, err := v1.NewStakingTransactionFromBlockIndex(block, uint64(index))
		if err != nil {
			return nil, err
		}
		return NewStructuredResponse(tx)
	case V2:
		tx, err := v2.NewStakingTransactionFromBlockIndex(block, uint64(index))
		if err != nil {
			return nil, err
		}
		return NewStructuredResponse(tx)
	default:
		return nil, ErrUnknownRPCVersion
	}
}

// GetTransactionReceipt returns the transaction receipt for the given transaction hash.
func (s *PublicTransactionService) GetTransactionReceipt(
	ctx context.Context, hash common.Hash,
) (StructuredResponse, error) {
	// Fetch receipt for plain & staking transaction
	var tx *types.Transaction
	var stx *staking.StakingTransaction
	var blockHash common.Hash
	var blockNumber, index uint64
	tx, blockHash, blockNumber, index = rawdb.ReadTransaction(s.itc.ChainDb(), hash)
	if tx == nil {
		stx, blockHash, blockNumber, index = rawdb.ReadStakingTransaction(s.itc.ChainDb(), hash)
		if stx == nil {
			return nil, nil
		}
		// if there both normal and staking transactions, add to index
		if block, _ := s.itc.GetBlock(ctx, blockHash); block != nil {
			index = index + uint64(block.Transactions().Len())
		}
	}
	receipts, err := s.itc.GetReceipts(ctx, blockHash)
	if err != nil {
		return nil, err
	}
	if len(receipts) <= int(index) {
		return nil, nil
	}
	receipt := receipts[index]

	// Format response according to version
	var RPCReceipt interface{}
	switch s.version {
	case V1:
		if tx == nil {
			RPCReceipt, err = v1.NewReceipt(stx, blockHash, blockNumber, index, receipt)
		} else {
			RPCReceipt, err = v1.NewReceipt(tx, blockHash, blockNumber, index, receipt)
		}
		if err != nil {
			return nil, err
		}
		return NewStructuredResponse(RPCReceipt)
	case V2:
		if tx == nil {
			RPCReceipt, err = v2.NewReceipt(stx, blockHash, blockNumber, index, receipt)
		} else {
			RPCReceipt, err = v2.NewReceipt(tx, blockHash, blockNumber, index, receipt)
		}
		if err != nil {
			return nil, err
		}
		return NewStructuredResponse(RPCReceipt)
	case Eth:
		if tx == nil {
			RPCReceipt, err = eth.NewReceipt(stx, blockHash, blockNumber, index, receipt)
		} else {
			RPCReceipt, err = eth.NewReceipt(tx, blockHash, blockNumber, index, receipt)
		}
		if err != nil {
			return nil, err
		}
		return NewStructuredResponse(RPCReceipt)
	default:
		return nil, ErrUnknownRPCVersion
	}
}

// GetCXReceiptByHash returns the transaction for the given hash
func (s *PublicTransactionService) GetCXReceiptByHash(
	ctx context.Context, hash common.Hash,
) (StructuredResponse, error) {
	if cx, blockHash, blockNumber, _ := rawdb.ReadCXReceipt(s.itc.ChainDb(), hash); cx != nil {
		// Format response according to version
		switch s.version {
		case V1, Eth:
			tx, err := v1.NewCxReceipt(cx, blockHash, blockNumber)
			if err != nil {
				return nil, err
			}
			return NewStructuredResponse(tx)
		case V2:
			tx, err := v2.NewCxReceipt(cx, blockHash, blockNumber)
			if err != nil {
				return nil, err
			}
			return NewStructuredResponse(tx)
		default:
			return nil, ErrUnknownRPCVersion
		}
	}
	utils.Logger().Debug().
		Err(fmt.Errorf("unable to found CX receipt for tx %v", hash.String())).
		Msgf("%v error at %v", LogTag, "GetCXReceiptByHash")
	return nil, nil // Legacy behavior is to not return an error here
}

// ResendCx requests that the egress receipt for the given cross-shard
// transaction be sent to the destination shard for credit.  This is used for
// unblocking a half-complete cross-shard transaction whose fund has been
// withdrawn already from the source shard but not credited yet in the
// destination account due to transient failures.
func (s *PublicTransactionService) ResendCx(ctx context.Context, txID common.Hash) (bool, error) {
	_, success := s.itc.ResendCx(ctx, txID)

	// Response output is the same for all versions
	return success, nil
}

// returnHashesWithPagination returns result with pagination (offset, page in TxHistoryArgs).
func returnHashesWithPagination(hashes []common.Hash, pageIndex uint32, pageSize uint32) []common.Hash {
	size := defaultPageSize
	if pageSize > 0 {
		size = pageSize
	}
	if uint64(size)*uint64(pageIndex) >= uint64(len(hashes)) {
		return make([]common.Hash, 0)
	}
	if uint64(size)*uint64(pageIndex)+uint64(size) > uint64(len(hashes)) {
		return hashes[size*pageIndex:]
	}
	return hashes[size*pageIndex : size*pageIndex+size]
}

// EstimateGas ..
// TODO: fix contract creation gas estimation, it currently underestimates.
func EstimateGas(
	ctx context.Context, itc *itc.Intelchain, args CallArgs, gasCap *big.Int,
) (uint64, error) {
	// Binary search the gas requirement, as it may be higher than the amount used
	var (
		lo  = params.TxGas - 1
		hi  uint64
		max uint64
	)
	blockNum := rpc.LatestBlockNumber
	if args.Gas != nil && uint64(*args.Gas) >= params.TxGas {
		hi = uint64(*args.Gas)
	} else {
		// Retrieve the blk to act as the gas ceiling
		blk, err := itc.BlockByNumber(ctx, blockNum)
		if err != nil {
			return 0, err
		}
		hi = blk.GasLimit()
	}
	if gasCap != nil && hi > gasCap.Uint64() {
		hi = gasCap.Uint64()
	}
	max = hi

	// Use zero-address if none other is available
	if args.From == nil {
		args.From = &common.Address{}
	}

	// Create a helper to check if a gas allowance results in an executable transaction
	executable := func(gas uint64) bool {
		args.Gas = (*hexutil.Uint64)(&gas)

		result, err := DoEVMCall(ctx, itc, args, blockNum, 0)
		if err != nil || result.VMErr == vm.ErrCodeStoreOutOfGas || result.VMErr == vm.ErrOutOfGas {
			return false
		}
		return true
	}

	// Execute the binary search and hone in on an executable gas limit
	for lo+1 < hi {
		mid := (hi + lo) / 2
		if !executable(mid) {
			lo = mid
		} else {
			hi = mid
		}
	}

	// Reject the transaction as invalid if it still fails at the highest allowance
	if hi == max {
		if !executable(hi) {
			return 0, fmt.Errorf("gas required exceeds allowance (%d) or always failing transaction", max)
		}
	}
	return hi, nil
}
