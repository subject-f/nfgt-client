package api

import (
	"errors"
	"fmt"
	"nfgt-client/pkg/client"
	"nfgt-client/pkg/common"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const (
	TRANSACTION_PARAM = "transactionId"
	OWNER_PARAM       = "ownerId"
	ASSET_PARAM       = "assetId"
	DEPTH_PARAM       = "depth"
)

func (r *RouterController) healthCheck(c *gin.Context) {
	c.JSON(200, gin.H{
		"message": "ok",
	})
}

type Transaction struct {
	OwnerId    string                 `json:"owner_id"`
	AssetId    string                 `json:"asset_id"`
	Passphrase string                 `json:"passphrase"`
	Metadata   map[string]interface{} `json:"metadata"`
}

func (r *RouterController) createTransaction(c *gin.Context) {
	var transaction Transaction
	c.BindJSON(&transaction)
	pendingTransaction, err := r.client.Transact(transaction.Passphrase, client.Transaction{
		TransactionId:   uuid.NewString(),
		TransactionTime: int(time.Now().UnixMilli()),
		Asset: client.Asset{
			AssetId:  transaction.AssetId,
			OwnerId:  transaction.OwnerId,
			Metadata: transaction.Metadata,
		},
	})

	switch {
	case errors.Is(err, client.ErrTransactionIdExists):
		common.Warnf("Transaction ID collision\n")
		c.JSON(500, gin.H{
			"error": "The internally generated transaction ID already exists.",
		})
	case errors.Is(err, client.ErrConcurrentAssetId):
		common.Warnf("Concurrent asset ID transaction\n")
		c.JSON(409, gin.H{
			"error": fmt.Sprintf("The asset ID %v is pending a transaction.", transaction.AssetId),
		})
	case errors.Is(err, client.ErrPassphase):
		common.Warnf("The passphrase was incorrect\n")
		c.JSON(403, gin.H{
			"error": "The submitted passphrase was incorrect.",
		})
	case errors.Is(err, client.ErrTransactionCommit):
		common.Warnf("Transaction commit failed, this is an internal git error.\n")
		c.JSON(500, gin.H{
			"error": "The transaction failed to commit.",
		})
	case err != nil:
		common.Warnf("Uncaught error: %v\n", err)
		c.JSON(500, gin.H{
			"error": err.Error(),
		})
	default:
		c.JSON(200, gin.H{
			"transaction": pendingTransaction,
		})
	}
}

func (r *RouterController) checkTransaction(c *gin.Context) {
	transactionId := c.Param(TRANSACTION_PARAM)
	c.JSON(200, gin.H{
		"committed": r.client.IsTransactionCommitted(transactionId),
		"rejected":  r.client.IsTransactionRejected(transactionId),
	})
}

func (r *RouterController) getTransactionDetails(c *gin.Context) {
	transactionId := c.Param(TRANSACTION_PARAM)
	c.JSON(200, gin.H{
		"transaction": r.client.GetTransaction(transactionId),
	})
}

func (r *RouterController) getOwnerSpot(c *gin.Context) {
	ownerId := c.Param(OWNER_PARAM)

	c.JSON(200, gin.H{
		"assets": r.client.GetOwnerAssets(ownerId),
	})
}

func (r *RouterController) getAssetSpot(c *gin.Context) {
	assetId := c.Param(ASSET_PARAM)

	transactions := r.client.GetAssetTransactionHistory(assetId, 1)

	if len(transactions) > 0 {
		c.JSON(200, gin.H{
			"transaction": transactions[0],
		})
	} else {
		c.JSON(200, gin.H{
			"transaction": nil,
		})
	}
}

func (r *RouterController) getOwnerHistory(c *gin.Context) {
	ownerId := c.Param(OWNER_PARAM)
	depth, err := parseDepth(c.Param(DEPTH_PARAM))

	if err != nil {
		c.JSON(400, gin.H{
			"error": "Failed to parse depth parameter.",
		})
		return
	}

	transactions := r.client.GetOwnerTransactionHistory(ownerId, int(depth))

	c.JSON(200, gin.H{
		"transactions": transactions,
	})
}

func (r *RouterController) getAssetHistory(c *gin.Context) {
	assetId := c.Param(ASSET_PARAM)
	depth, err := parseDepth(c.Param(DEPTH_PARAM))

	if err != nil {
		c.JSON(400, gin.H{
			"error": "Failed to parse depth parameter.",
		})
		return
	}

	transactions := r.client.GetAssetTransactionHistory(assetId, int(depth))

	c.JSON(200, gin.H{
		"transactions": transactions,
	})
}

func parseDepth(depth string) (int64, error) {
	return strconv.ParseInt(depth, 10, 64)
}
