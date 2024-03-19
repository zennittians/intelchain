package services

import (
	"context"
	"fmt"
	"math/big"
	"time"

	"github.com/coinbase/rosetta-sdk-go/server"
	"github.com/coinbase/rosetta-sdk-go/types"
	ethcommon "github.com/ethereum/go-ethereum/common"
	lru "github.com/hashicorp/golang-lru"

	"github.com/zennittians/intelchain/core/rawdb"
	itctypes "github.com/zennittians/intelchain/core/types"
	"github.com/zennittians/intelchain/core/vm"
	"github.com/zennittians/intelchain/itc"
	"github.com/zennittians/intelchain/rosetta/common"
	"github.com/zennittians/intelchain/rpc"
	stakingTypes "github.com/zennittians/intelchain/staking/types"
)

const (
	// txTraceCacheSize is max number of transaction traces to keep cached
	txTraceCacheSize = 1e5
)

// BlockAPI implements the server.BlockAPIServicer interface.
type BlockAPI struct {
	itc          *itc.Intelchain
	txTraceCache *lru.Cache
}

// NewBlockAPI creates a new instance of a BlockAPI.
func NewBlockAPI(itc *itc.Intelchain) server.BlockAPIServicer {
	traceCache, _ := lru.New(txTraceCacheSize)
	return &BlockAPI{
		itc:          itc,
		txTraceCache: traceCache,
	}
}

// BlockMetadata ..
type BlockMetadata struct {
	Epoch *big.Int `json:"epoch"`
}

// Block implements the /block endpoint
func (s *BlockAPI) Block(
	ctx context.Context, request *types.BlockRequest,
) (response *types.BlockResponse, rosettaError *types.Error) {
	if err := assertValidNetworkIdentifier(request.NetworkIdentifier, s.itc.ShardID); err != nil {
		return nil, err
	}

	var blk *itctypes.Block
	var currBlockID, prevBlockID *types.BlockIdentifier
	if blk, rosettaError = getBlock(ctx, s.itc, request.BlockIdentifier); rosettaError != nil {
		return nil, rosettaError
	}

	currBlockID = &types.BlockIdentifier{
		Index: blk.Number().Int64(),
		Hash:  blk.Hash().String(),
	}

	if blk.NumberU64() == 0 {
		prevBlockID = currBlockID
	} else {
		prevBlock, err := s.itc.BlockByNumber(ctx, rpc.BlockNumber(blk.Number().Int64()-1).EthBlockNumber())
		if err != nil {
			return nil, common.NewError(common.CatchAllError, map[string]interface{}{
				"message": err.Error(),
			})
		}
		prevBlockID = &types.BlockIdentifier{
			Index: prevBlock.Number().Int64(),
			Hash:  prevBlock.Hash().String(),
		}
	}

	// Report any side effect transaction now as it can be computed & cached on block fetch.
	transactions := []*types.Transaction{}
	if s.containsSideEffectTransaction(ctx, blk) {
		tx, rosettaError := s.getSideEffectTransaction(ctx, blk)
		if rosettaError != nil {
			return nil, rosettaError
		}
		transactions = append(transactions, tx)
	}

	metadata, err := types.MarshalMap(BlockMetadata{
		Epoch: blk.Epoch(),
	})
	if err != nil {
		return nil, common.NewError(common.CatchAllError, map[string]interface{}{
			"message": err.Error(),
		})
	}
	responseBlock := &types.Block{
		BlockIdentifier:       currBlockID,
		ParentBlockIdentifier: prevBlockID,
		Timestamp:             blk.Time().Int64() * 1e3, // Timestamp must be in ms.
		Transactions:          transactions,
		Metadata:              metadata,
	}

	otherTransactions := []*types.TransactionIdentifier{}
	for _, tx := range blk.Transactions() {
		otherTransactions = append(otherTransactions, &types.TransactionIdentifier{
			Hash: tx.Hash().String(),
		})
	}
	for _, tx := range blk.StakingTransactions() {
		otherTransactions = append(otherTransactions, &types.TransactionIdentifier{
			Hash: tx.Hash().String(),
		})
	}
	// Report cross-shard transaction payouts.
	for _, cxReceipts := range blk.IncomingReceipts() {
		for _, cxReceipt := range cxReceipts.Receipts {
			otherTransactions = append(otherTransactions, &types.TransactionIdentifier{
				Hash: cxReceipt.TxHash.String(),
			})
		}
	}

	return &types.BlockResponse{
		Block:             responseBlock,
		OtherTransactions: otherTransactions,
	}, nil
}

// BlockTransaction implements the /block/transaction endpoint
func (s *BlockAPI) BlockTransaction(
	ctx context.Context, request *types.BlockTransactionRequest,
) (*types.BlockTransactionResponse, *types.Error) {
	if err := assertValidNetworkIdentifier(request.NetworkIdentifier, s.itc.ShardID); err != nil {
		return nil, err
	}

	blk, rosettaError := getBlock(
		ctx, s.itc, &types.PartialBlockIdentifier{Hash: &request.BlockIdentifier.Hash},
	)
	if rosettaError != nil {
		return nil, rosettaError
	}
	txHash := ethcommon.HexToHash(request.TransactionIdentifier.Hash)
	txInfo, rosettaError := s.getTransactionInfo(ctx, blk, txHash)
	if rosettaError != nil {
		// If no transaction info is found, check for side effect case transaction.
		response, rosettaError2 := s.sideEffectBlockTransaction(ctx, request)
		if rosettaError2 != nil && rosettaError2.Code != common.TransactionNotFoundError.Code {
			return nil, common.NewError(common.TransactionNotFoundError, map[string]interface{}{
				"from_error": rosettaError2,
			})
		}
		return response, rosettaError2
	}
	state, _, err := s.itc.StateAndHeaderByNumber(ctx, rpc.BlockNumber(request.BlockIdentifier.Index).EthBlockNumber())
	if state == nil || err != nil {
		return nil, common.NewError(common.BlockNotFoundError, map[string]interface{}{
			"message": fmt.Sprintf("block state not found for block %v", request.BlockIdentifier.Index),
		})
	}

	var transaction *types.Transaction
	if txInfo.tx != nil && txInfo.receipt != nil {
		contractInfo := &ContractInfo{}
		if _, ok := txInfo.tx.(*itctypes.Transaction); ok {
			// check for contract related operations, if it is a plain transaction.
			if txInfo.tx.To() != nil {
				// possible call to existing contract so fetch relevant data
				contractInfo.ContractCode = state.GetCode(*txInfo.tx.To())
				contractInfo.ContractAddress = txInfo.tx.To()
			} else {
				// contract creation, so address is in receipt
				contractInfo.ContractCode = state.GetCode(txInfo.receipt.ContractAddress)
				contractInfo.ContractAddress = &txInfo.receipt.ContractAddress
			}
			contractInfo.ExecutionResult, rosettaError = s.getTransactionTrace(ctx, blk, txInfo)
			if rosettaError != nil {
				return nil, rosettaError
			}
		}
		transaction, rosettaError = FormatTransaction(txInfo.tx, txInfo.receipt, contractInfo)
		if rosettaError != nil {
			return nil, rosettaError
		}
	} else if txInfo.cxReceipt != nil {
		transaction, rosettaError = FormatCrossShardReceiverTransaction(txInfo.cxReceipt)
		if rosettaError != nil {
			return nil, rosettaError
		}
	} else {
		return nil, &common.TransactionNotFoundError
	}
	return &types.BlockTransactionResponse{Transaction: transaction}, nil
}

// transactionInfo stores all related information for any transaction on the Intelchain chain
// Note that some elements can be nil if not applicable
type transactionInfo struct {
	tx        itctypes.PoolTransaction
	txIndex   uint64
	receipt   *itctypes.Receipt
	cxReceipt *itctypes.CXReceipt
}

// getTransactionInfo given the block hash and transaction hash
func (s *BlockAPI) getTransactionInfo(
	ctx context.Context, blk *itctypes.Block, txHash ethcommon.Hash,
) (txInfo *transactionInfo, rosettaError *types.Error) {
	// Look for all of the possible transactions
	var index uint64
	var plainTx *itctypes.Transaction
	var stakingTx *stakingTypes.StakingTransaction
	plainTx, _, _, index = rawdb.ReadTransaction(s.itc.ChainDb(), txHash)
	if plainTx == nil {
		stakingTx, _, _, index = rawdb.ReadStakingTransaction(s.itc.ChainDb(), txHash)
		// if there both normal and staking transactions, correct index offset.
		index = index + uint64(blk.Transactions().Len())
	}
	cxReceipt, _, _, _ := rawdb.ReadCXReceipt(s.itc.ChainDb(), txHash)

	if plainTx == nil && stakingTx == nil && cxReceipt == nil {
		return nil, &common.TransactionNotFoundError
	}

	var receipt *itctypes.Receipt
	receipts, err := s.itc.GetReceipts(ctx, blk.Hash())
	if err != nil {
		return nil, common.NewError(common.CatchAllError, map[string]interface{}{
			"message": err.Error(),
		})
	}
	if int(index) < len(receipts) {
		receipt = receipts[index]
		if cxReceipt == nil && receipt.TxHash != txHash {
			return nil, common.NewError(common.CatchAllError, map[string]interface{}{
				"message": "unable to find correct receipt for transaction",
			})
		}
	}

	// Use pool transaction for concise formatting
	var tx itctypes.PoolTransaction
	if stakingTx != nil {
		tx = stakingTx
	} else if plainTx != nil {
		tx = plainTx
	}

	return &transactionInfo{
		tx:        tx,
		txIndex:   index,
		receipt:   receipt,
		cxReceipt: cxReceipt,
	}, nil
}

var (
	// defaultTraceReExec is the number of blocks the tracer can go back and re-execute to produce historical state.
	// Only 1 block is needed to check for internal transactions
	defaultTraceReExec = uint64(1)
	// defaultTraceTimeout is the amount of time a transaction can execute
	defaultTraceTimeout = (10 * time.Second).String()
	// defaultTraceLogConfig is the log config of all traces
	defaultTraceLogConfig = vm.LogConfig{
		DisableMemory:  false,
		DisableStack:   false,
		DisableStorage: false,
		Debug:          false,
		Limit:          0,
	}
)

// getTransactionTrace for the given txInfo.
func (s *BlockAPI) getTransactionTrace(
	ctx context.Context, blk *itctypes.Block, txInfo *transactionInfo,
) (*itc.ExecutionResult, *types.Error) {
	cacheKey := types.Hash(blk) + types.Hash(txInfo)
	if value, ok := s.txTraceCache.Get(cacheKey); ok {
		return value.(*itc.ExecutionResult), nil
	}

	msg, vmctx, statedb, err := s.itc.ComputeTxEnv(blk, int(txInfo.txIndex), defaultTraceReExec)
	if err != nil {
		return nil, common.NewError(common.CatchAllError, map[string]interface{}{
			"message": err.Error(),
		})
	}
	execResultInterface, err := s.itc.TraceTx(ctx, msg, vmctx, statedb, &itc.TraceConfig{
		LogConfig: &defaultTraceLogConfig,
		Timeout:   &defaultTraceTimeout,
		Reexec:    &defaultTraceReExec,
	})
	if err != nil {
		return nil, common.NewError(common.CatchAllError, map[string]interface{}{
			"message": err.Error(),
		})
	}
	execResult, ok := execResultInterface.(*itc.ExecutionResult)
	if !ok {
		return nil, common.NewError(common.CatchAllError, map[string]interface{}{
			"message": "unknown tracer exec result type",
		})
	}
	s.txTraceCache.Add(cacheKey, execResult)

	return execResult, nil
}

// getBlock ..
func getBlock(
	ctx context.Context, itc *itc.Intelchain, blockID *types.PartialBlockIdentifier,
) (blk *itctypes.Block, rosettaError *types.Error) {
	var err error
	if blockID.Hash != nil {
		requestBlockHash := ethcommon.HexToHash(*blockID.Hash)
		blk, err = itc.GetBlock(ctx, requestBlockHash)
	} else if blockID.Index != nil {
		blk, err = itc.BlockByNumber(ctx, rpc.BlockNumber(*blockID.Index).EthBlockNumber())
	} else {
		return nil, &common.BlockNotFoundError
	}
	if err != nil {
		return nil, common.NewError(common.BlockNotFoundError, map[string]interface{}{
			"message": err.Error(),
		})
	}
	if blk == nil {
		return nil, common.NewError(common.BlockNotFoundError, map[string]interface{}{
			"message": "block not found for given block identifier",
		})
	}
	return blk, nil
}
