package tx

import (
	"fmt"
	"log/slog"
	"math/big"
	"sync"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	hdconfig "github.com/nodeset-org/hyperdrive-daemon/shared/config"
	"github.com/rocket-pool/node-manager-core/config"
	"github.com/rocket-pool/node-manager-core/eth"
	"golang.org/x/sync/errgroup"
)

// Prints a TX's details to the logger and waits for it to validated.
func PrintAndWaitForTransaction(res *config.NetworkResources, txMgr *eth.TransactionManager, logger *slog.Logger, txInfo *eth.TransactionInfo, opts *bind.TransactOpts) error {
	tx, err := txMgr.ExecuteTransaction(txInfo, opts)
	if err != nil {
		return fmt.Errorf("error submitting transaction: %w", err)
	}

	txWatchUrl := res.TxWatchUrl
	hashString := tx.Hash().String()
	logger.Info(
		"Transaction has been submitted.",
		slog.String("hash", hashString),
	)
	if txWatchUrl != "" {
		logger.Info("You may follow its progress by visiting:")
		logger.Info(fmt.Sprintf("%s/%s\n", txWatchUrl, hashString))
	}

	// Wait for the TX to be included in a block
	logger.Info("Waiting for the transaction to be validated...")
	err = txMgr.WaitForTransaction(tx)
	if err != nil {
		return fmt.Errorf("error waiting for transaction: %w", err)
	}

	return nil
}

// Prints a TX's details to the logger and waits for it to validated.
func PrintAndWaitForTransactionBatch(res *config.NetworkResources, txMgr *eth.TransactionManager, logger *slog.Logger, submissions []*eth.TransactionSubmission, opts *bind.TransactOpts) error {
	txs, err := txMgr.BatchExecuteTransactions(submissions, opts)
	if err != nil {
		return fmt.Errorf("error submitting transactions: %w", err)
	}

	txWatchUrl := res.TxWatchUrl
	if txWatchUrl != "" {
		logger.Info("Transactions have been submitted. You may follow them progress by visiting:")
		for _, tx := range txs {
			hashString := tx.Hash().String()
			logger.Info(fmt.Sprintf("%s/%s\n", txWatchUrl, hashString))
		}
	} else {
		logger.Info("Transactions have been submitted with the following hashes:")
		for _, tx := range txs {
			logger.Info(tx.Hash().String())
		}

	}

	// Wait for the TX to be included in a block
	logger.Info("Waiting for the transactions to be validated...")
	var wg errgroup.Group
	var waitLock sync.Mutex
	completeCount := 0

	for _, tx := range txs {
		tx := tx
		wg.Go(func() error {
			err := txMgr.WaitForTransaction(tx)
			if err != nil {
				err = fmt.Errorf("error waiting for transaction %s: %w", tx.Hash().String(), err)
			} else {
				waitLock.Lock()
				completeCount++
				logger.Info(fmt.Sprintf("TX %s complete (%d/%d)", tx.Hash().String(), completeCount, len(txs)))
				waitLock.Unlock()
			}
			return err
		})
	}

	err = wg.Wait()
	if err != nil {
		return err
	}

	logger.Info("Transaction batch complete.")
	return nil
}

// Gets the automatic TX max fee and max priority fee for daemon processes
func GetAutoTxInfo(cfg *hdconfig.HyperdriveConfig, logger *slog.Logger) (*big.Int, *big.Int) {
	// Get the user-requested max fee
	maxFeeGwei := cfg.AutoTxMaxFee.Value
	var maxFee *big.Int
	if maxFeeGwei == 0 {
		maxFee = nil
	} else {
		maxFee = eth.GweiToWei(maxFeeGwei)
	}

	// Get the user-requested max fee
	priorityFeeGwei := cfg.MaxPriorityFee.Value
	var priorityFee *big.Int
	if priorityFeeGwei == 0 {
		logger.Warn("Priority fee was missing or 0, setting a default of 2.")
		priorityFee = eth.GweiToWei(2)
	} else {
		priorityFee = eth.GweiToWei(priorityFeeGwei)
	}

	return maxFee, priorityFee
}
