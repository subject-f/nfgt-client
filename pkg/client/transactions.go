package client

import (
	"fmt"
)

type Asset struct {
	AssetId  string                 `json:"asset_id"`
	OwnerId  string                 `json:"owner_id"`
	Metadata map[string]interface{} `json:"metadata"`
}

type Transaction struct {
	TransactionId   string `json:"transaction_id"`
	TransactionTime int    `json:"transaction_time"`
	Asset
}

// A transaction becomes verified when it has a successor hash.
// The requirement for such is that the predecessor has been verified,
// and a new passphrase was generated and hashed for a given transaction.
type VerifiedTransaction struct {
	SuccessorHash string `json:"successor_hash"`
	Transaction
}

type PendingTransaction struct {
	SuccessorPassphrase string
	VerifiedTransaction
}

type TransactionType int64

const (
	NFGT TransactionType = iota
)

const (
	refPrefix     = "refs/"
	refNfgtPrefix = "nfgt/"
	refTagPrefix  = ""
)

func GetTransactionBranch(transactionType TransactionType, transaction Transaction) string {
	switch transactionType {
	case NFGT:
		return fmt.Sprintf("%v/%v", "nfgt", transaction.AssetId)
	default:
		panic("transaction type not recognized")
	}
}

func GetAssetIdBranch(transactionType TransactionType, assetId string) string {
	switch transactionType {
	case NFGT:
		return fmt.Sprintf("%v/%v", "nfgt", assetId)
	default:
		panic("transaction type not recognized")
	}
}
