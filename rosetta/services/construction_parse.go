package services

import (
	"context"
	"fmt"

	"github.com/coinbase/rosetta-sdk-go/types"
	"github.com/pkg/errors"

	itcTypes "github.com/zennittians/intelchain/core/types"
	"github.com/zennittians/intelchain/rosetta/common"
)

// ConstructionParse implements the /construction/parse endpoint.
func (s *ConstructAPI) ConstructionParse(
	ctx context.Context, request *types.ConstructionParseRequest,
) (*types.ConstructionParseResponse, *types.Error) {
	if err := assertValidNetworkIdentifier(request.NetworkIdentifier, s.itc.ShardID); err != nil {
		return nil, err
	}
	wrappedTransaction, tx, rosettaError := unpackWrappedTransactionFromString(request.Transaction)
	if rosettaError != nil {
		return nil, rosettaError
	}
	if tx.ShardID() != s.itc.ShardID {
		return nil, common.NewError(common.InvalidTransactionConstructionError, map[string]interface{}{
			"message": fmt.Sprintf("transaction is for shard %v != shard %v", tx.ShardID(), s.itc.ShardID),
		})
	}
	if request.Signed {
		return parseSignedTransaction(ctx, wrappedTransaction, tx)
	}
	return parseUnsignedTransaction(ctx, wrappedTransaction, tx)
}

// parseUnsignedTransaction ..
func parseUnsignedTransaction(
	ctx context.Context, wrappedTransaction *WrappedTransaction, tx itcTypes.PoolTransaction,
) (*types.ConstructionParseResponse, *types.Error) {
	if wrappedTransaction == nil || tx == nil {
		return nil, common.NewError(common.CatchAllError, map[string]interface{}{
			"message": "nil wrapped transaction or unwrapped transaction",
		})
	}

	if _, err := getAddress(wrappedTransaction.From); err != nil {
		return nil, common.NewError(common.InvalidTransactionConstructionError, map[string]interface{}{
			"message": err.Error(),
		})
	}

	// TODO (dm): implement intended receipt for staking transactions
	intendedReceipt := &itcTypes.Receipt{
		GasUsed: tx.GasLimit(),
	}
	formattedTx, rosettaError := FormatTransaction(
		tx, intendedReceipt, &ContractInfo{ContractCode: wrappedTransaction.ContractCode},
	)
	if rosettaError != nil {
		return nil, rosettaError
	}
	tempAccID, rosettaError := newAccountIdentifier(FormatDefaultSenderAddress)
	if rosettaError != nil {
		return nil, rosettaError
	}
	foundSender := false
	operations := formattedTx.Operations
	for _, op := range operations {
		if op.Account.Address == tempAccID.Address {
			foundSender = true
			op.Account = wrappedTransaction.From
		}
		op.Status = ""
	}
	if !foundSender {
		return nil, common.NewError(common.CatchAllError, map[string]interface{}{
			"message": "temp sender not found in transaction operations",
		})
	}
	return &types.ConstructionParseResponse{
		Operations: operations,
	}, nil
}

// parseSignedTransaction ..
func parseSignedTransaction(
	ctx context.Context, wrappedTransaction *WrappedTransaction, tx itcTypes.PoolTransaction,
) (*types.ConstructionParseResponse, *types.Error) {
	if wrappedTransaction == nil || tx == nil {
		return nil, common.NewError(common.CatchAllError, map[string]interface{}{
			"message": "nil wrapped transaction or unwrapped transaction",
		})
	}

	// TODO (dm): implement intended receipt for staking transactions
	intendedReceipt := &itcTypes.Receipt{
		GasUsed: tx.GasLimit(),
	}
	formattedTx, rosettaError := FormatTransaction(
		tx, intendedReceipt, &ContractInfo{ContractCode: wrappedTransaction.ContractCode},
	)
	if rosettaError != nil {
		return nil, rosettaError
	}
	sender, err := tx.SenderAddress()
	if err != nil {
		return nil, common.NewError(common.InvalidTransactionConstructionError, map[string]interface{}{
			"message": errors.WithMessage(err, "unable to get sender address for signed transaction").Error(),
		})
	}
	senderID, rosettaError := newAccountIdentifier(sender)
	if rosettaError != nil {
		return nil, rosettaError
	}
	if senderID.Address != wrappedTransaction.From.Address {
		return nil, common.NewError(common.InvalidTransactionConstructionError, map[string]interface{}{
			"message": "wrapped transaction sender/from does not match transaction signer",
		})
	}
	for _, op := range formattedTx.Operations {
		op.Status = ""
	}
	return &types.ConstructionParseResponse{
		Operations:               formattedTx.Operations,
		AccountIdentifierSigners: []*types.AccountIdentifier{senderID},
	}, nil
}
