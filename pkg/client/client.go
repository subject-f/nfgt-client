package client

import (
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"time"

	"nfgt-client/pkg/common"
	"nfgt-client/pkg/git"

	"github.com/go-git/go-git/v5/plumbing"
)

var (
	ErrTransactionIdExists  = errors.New("transaction ID exists")
	ErrConcurrentAssetId    = errors.New("asset ID is currently being processed")
	ErrTransactionIdPending = errors.New("a transaction with the same ID is pending")
	ErrPassphase            = errors.New("could not verify the passphrase to process proceeding transaction")
	ErrJson                 = errors.New("failed to marshal JSON data")
	ErrTransactionCommit    = errors.New("failed to commit transaction")
	ErrSyncFailed           = errors.New("failed to sync chain state")
)

const (
	passphraseLength = 16
)

// The cilent should parse errors, do the checking before commiting a transaction,
// and acting like a source of truth for the state of the repo
type Client struct {
	RefLockSet              map[string]struct{}            // A set of all pending references
	TransactionSet          map[string]struct{}            // A set of the verified transaction IDs
	AssetIdMap              map[string]string              // Mapping of asset ID to owner ID
	OwnerSet                map[string]map[string]struct{} // A set of the asset IDs an owner currently has
	OwnerTransactionHistory map[string]map[string]struct{} // A map to a set of all transactions related to a given owner
	config                  common.Config
	GitProvider             *git.GitProvider
	sync.RWMutex
}

func NewClient(config common.Config, gitProvider *git.GitProvider) *Client {
	client := Client{
		RefLockSet:              make(map[string]struct{}),
		TransactionSet:          make(map[string]struct{}),
		AssetIdMap:              make(map[string]string),
		OwnerSet:                make(map[string]map[string]struct{}),
		OwnerTransactionHistory: make(map[string]map[string]struct{}),
		config:                  config,
		GitProvider:             gitProvider,
	}
	client.Sync()
	return &client
}

// Transacts a given transaction by verifying the rights (according to the predecessor password), then
// pushing the contents. The caller should not expect that the transaction is verified until they've
// manually checked it.
func (c *Client) Transact(predecessorPassphrase string, transaction Transaction) (*PendingTransaction, error) {
	c.RLock() // Unlock isn't deferred since we want to unlock ASAP
	start := time.Now()

	if _, exists := c.TransactionSet[transaction.TransactionId]; exists {
		c.RUnlock()
		common.Debugf("Transaction ID %v already exists\n", transaction.TransactionId)
		return nil, ErrTransactionIdExists
	}

	if _, exists := c.RefLockSet[transaction.AssetId]; exists {
		c.RUnlock()
		common.Debugf("A transaction with the current asset ID already exists\n")
		return nil, ErrConcurrentAssetId
	} else {
		c.RefLockSet[transaction.AssetId] = struct{}{}
	}
	c.RUnlock()

	worktree := c.GitProvider.CreateNewWorktree()

	shouldCheckPassphrase := true

	transactionBranch := GetTransactionBranch(NFGT, transaction)

	if worktree.CheckoutBranch(plumbing.NewRemoteReferenceName(c.config.RemoteName, transactionBranch)) != nil {
		common.Debugf("Starting a new transaction chain for %v\n", transactionBranch)
		worktree.CheckoutBranch(plumbing.NewRemoteReferenceName(c.config.RemoteName, git.BaseBranchName))
		shouldCheckPassphrase = false
	}

	if shouldCheckPassphrase {
		metadataStr, err := worktree.ReadMetadata()

		if common.CheckError(err) {
			return nil, ErrPassphase
		}

		var previousTransaction VerifiedTransaction

		json.Unmarshal([]byte(metadataStr), &previousTransaction)

		if previousTransaction.SuccessorHash != common.ComputeHash(predecessorPassphrase) {
			common.Debugf("Provided passphrase does match expected passphrase hash\n")
			return nil, ErrPassphase
		}
	}

	successorPassphrase := common.RandStringRunes(passphraseLength)
	pendingTransaction := PendingTransaction{
		SuccessorPassphrase: successorPassphrase,
		VerifiedTransaction: VerifiedTransaction{
			SuccessorHash: common.ComputeHash(successorPassphrase),
			Transaction:   transaction, // This might create a reference island, keep an eye on it
		},
	}

	data, err := json.Marshal(pendingTransaction.VerifiedTransaction)

	if common.CheckError(err) {
		return nil, ErrJson
	}

	hash, err := worktree.WriteAndCommitMetadata(string(data))

	if common.CheckError(err) {
		common.Debugf("Failed to commit transaction\n")
		return nil, ErrTransactionCommit
	}

	go func() {
		if c.GitProvider.Push(hash, transactionBranch, transaction.TransactionId) != nil {
			common.Warnf("Failed to push transaction %v\n", transaction.TransactionId)
		}
		c.Lock()
		delete(c.RefLockSet, transaction.AssetId)
		c.Unlock()
		common.Debugf("Transaction (%v) took %v\n", transaction.TransactionId, time.Since(start))
	}()

	return &pendingTransaction, nil
}

func (c *Client) IsTransactionCommitted(transactionId string) bool {
	c.RLock()
	defer c.RUnlock()

	_, ok := c.TransactionSet[transactionId]

	return ok
}

func (c *Client) GetAllAssets() []Asset {
	c.RLock()
	defer c.RUnlock()

	// TODO

	return nil
}

func (c *Client) GetAssetTransactionHistory(assetId string, depth int) []VerifiedTransaction {
	c.RLock()
	defer c.RUnlock()

	worktree := c.GitProvider.CreateNewWorktree()

	transactionBranch := GetAssetIdBranch(NFGT, assetId)
	refName := plumbing.NewRemoteReferenceName(c.config.RemoteName, transactionBranch)

	err := worktree.CheckoutBranch(refName)

	if common.CheckError(err) {
		return nil
	}

	transactionHistory := make([]VerifiedTransaction, 5)

	for i := 0; i < depth; i++ {
		err = worktree.CheckoutParent(i)
		if common.CheckError(err) {
			break
		}

		t, err := getVerifiedTransaction(worktree)

		if common.CheckError(err) {
			break
		}

		transactionHistory = append(transactionHistory, *t)
	}

	return transactionHistory
}

func (c *Client) GetOwnerAssets(ownerId string) []VerifiedTransaction {
	c.RLock()
	defer c.RUnlock()

	worktree := c.GitProvider.CreateNewWorktree()

	currentOwnerSet, exists := c.OwnerSet[ownerId]

	if !exists {
		return []VerifiedTransaction{}
	}

	assets := make([]VerifiedTransaction, 5)

	for assetId := range currentOwnerSet {
		transactionBranch := GetAssetIdBranch(NFGT, assetId)
		refName := plumbing.NewRemoteReferenceName(c.config.RemoteName, transactionBranch)

		err := worktree.CheckoutBranch(refName)

		if common.CheckError(err) {
			common.Warnf("Failed to checkout ref (%v) during sync\n", refName)
			continue
		}

		t, err := getVerifiedTransaction(worktree)

		if err != nil {
			continue
		}

		assets = append(assets, *t)
	}
	return assets
}

func (c *Client) GetOwnerTransactionHistory(ownerId string, depth int) []VerifiedTransaction {
	c.RLock()
	defer c.RUnlock()

	// TODO

	return nil
}

func (c *Client) Sync() error {
	worktree := c.GitProvider.CreateNewWorktree()

	c.GitProvider.Fetch()

	r, err := c.GitProvider.Repo.References()

	if common.CheckError(err) {
		return ErrSyncFailed
	}

	success := 0
	failure := 0

	r.ForEach(func(r *plumbing.Reference) error {
		err := worktree.CheckoutBranch(r.Name())

		if common.CheckError(err) {
			return nil
		}
		switch {
		// Branch HEADs track the current owners of the given asset, so we update our cache accordingly
		case strings.HasPrefix(r.Name().String(), plumbing.NewRemoteReferenceName(c.config.RemoteName, refNfgtPrefix).String()):
			t, err := getVerifiedTransaction(worktree)
			if common.CheckError(err) {
				failure += 1
			} else {
				c.processBranch(r.Name(), t)
			}
		// Tags are general transactions that we should process
		case strings.HasPrefix(r.Name().String(), plumbing.NewTagReferenceName(refTagPrefix).String()):
			t, err := getVerifiedTransaction(worktree)
			if common.CheckError(err) {
				failure += 1
			} else {
				c.processTag(r.Name(), t)
			}
		}

		success += 1
		return nil
	})

	common.Infof("Sync status (success: %v, failure: %v)\n", success, failure)
	return nil
}

func getVerifiedTransaction(worktree *git.GitWorktree) (*VerifiedTransaction, error) {
	fileContents, err := worktree.ReadMetadata()

	if common.CheckError(err) {
		return nil, err
	}
	var verifiedTransaction VerifiedTransaction
	err = json.Unmarshal([]byte(fileContents), &verifiedTransaction)

	if common.CheckError(err) {
		return nil, err
	}

	return &verifiedTransaction, nil
}

// Branch HEADs are special references that identify ownership at the time of sync. Update
// the local cache accordingly to transfer or establish ownership
func (c *Client) processBranch(refName plumbing.ReferenceName, vTransaction *VerifiedTransaction) error {
	c.Lock()
	defer c.Unlock()

	previousOwner, exists := c.AssetIdMap[vTransaction.AssetId]

	if exists {
		// If the current asset already had a previous owner, we'll have to update the references
		// This may fail if an asset went through multiple owners before the previous sync, but
		// that's okay since this is just for the local cache based on the previous owner
		delete(c.OwnerSet[previousOwner], vTransaction.AssetId)
	}
	c.AssetIdMap[vTransaction.AssetId] = vTransaction.OwnerId

	currentOwnerSet, exists := c.OwnerSet[vTransaction.OwnerId]

	if !exists {
		c.OwnerSet[vTransaction.OwnerId] = make(map[string]struct{})
		currentOwnerSet = c.OwnerSet[vTransaction.OwnerId]
	}

	currentOwnerSet[vTransaction.AssetId] = struct{}{}

	return nil
}

// Transactions aren't considered committed until they're tagged in the git repo, so we only "commit"
// a transaction to the local cache if we have a tag reference
func (c *Client) processTag(refName plumbing.ReferenceName, vTransaction *VerifiedTransaction) error {
	c.Lock()
	defer c.Unlock()

	c.TransactionSet[vTransaction.TransactionId] = struct{}{}

	ownerSet, exists := c.OwnerTransactionHistory[vTransaction.OwnerId]

	if !exists {
		c.OwnerTransactionHistory[vTransaction.OwnerId] = make(map[string]struct{})
		ownerSet = c.OwnerTransactionHistory[vTransaction.OwnerId]
	}

	ownerSet[vTransaction.TransactionId] = struct{}{}

	return nil
}