package git

import (
	"errors"
	"fmt"
	"io/ioutil"
	"nfgt-client/pkg/common"
	"time"

	"github.com/go-git/go-billy/v5/memfs"
	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"
	ssh2 "golang.org/x/crypto/ssh"
)

var (
	ErrWorktreeCreation   = errors.New("failed to create worktree")
	ErrMetadataRW         = errors.New("failed to read/write metadata")
	ErrMetadataCommit     = errors.New("failed to commit metadata file")
	ErrBranchDoesNotExist = errors.New("branch doesn't exist")
	ErrFailedToPush       = errors.New("failed to push reference, another writer may have pushed")
)

const (
	MetadataFilename string = "metadata.json"
	hashCutoffCount  int    = 6
)

type GitProvider struct {
	config          *common.Config
	storageProvider *StorageProvider
	auth            *ssh.PublicKeys
	Repo            *gogit.Repository
}

// Create a new GitProvider for a given config. In an ideal world, this would be a singleton
func NewGitProvider(config *common.Config) *GitProvider {
	common.Debugf("Initializing repository (%v) as backing chain.\n", config.RemoteUrl)

	auth, err := ssh.NewPublicKeys("git", []byte(config.SshKey), config.SshPassphrase)
	auth.HostKeyCallback = ssh2.InsecureIgnoreHostKey()

	common.CheckError(err)

	storageProvider := NewStorageProvider()
	storage := storageProvider.NewSharedStorage()
	worktree := memfs.New()

	repo, err := gogit.Clone(storage, worktree, &gogit.CloneOptions{
		RemoteName: config.RemoteName,
		URL:        config.RemoteUrl,
		Auth:       auth,
		NoCheckout: true,
	})

	common.CheckError(err)

	return &GitProvider{
		config:          config,
		storageProvider: storageProvider,
		auth:            auth,
		Repo:            repo,
	}
}

// Fetches from the remote
func (g *GitProvider) Fetch() error {
	remote, _ := g.Repo.Remote(g.config.RemoteName)

	return remote.Fetch(&gogit.FetchOptions{
		RemoteName: g.config.RemoteName,
		Tags:       gogit.AllTags,
		Auth:       g.auth,
	})
}

func (g *GitProvider) Push(hash plumbing.Hash, branch string, tag string) error {
	return g.Repo.Push(&gogit.PushOptions{
		RemoteName: g.config.RemoteName,
		RefSpecs: []config.RefSpec{
			config.RefSpec(fmt.Sprintf("%v:%v", hash.String(), plumbing.NewBranchReferenceName(branch))),
			config.RefSpec(fmt.Sprintf("%v:%v", hash.String(), plumbing.NewTagReferenceName(tag))),
		},
		Auth: g.auth,
		// Upon further investigation, go-git doesn't adhere to the git spec
		// _exactly_ since it checks all references before actually pushing,
		// preventing non-fast-foward updates. However, we'll make it more
		// explicit and correct by setting the flag to push atomically
		Atomic: true,
	})
}

// Creates a new thread-safe worktree for the given GitProvider
func (g *GitProvider) CreateNewWorktree() *GitWorktree {
	storage := g.storageProvider.NewSharedStorage()
	worktreeFs := memfs.New()

	repo, err := gogit.Open(storage, worktreeFs)

	common.CheckError(err)

	repo.CreateRemote(&config.RemoteConfig{
		Name: g.config.RemoteName,
		URLs: []string{g.config.RemoteUrl},
	})

	worktree, err := repo.Worktree()

	common.CheckError(err)

	return &GitWorktree{
		provider: g,
		config:   g.config,
		Repo:     repo,
		Worktree: worktree,
	}
}

type GitWorktree struct {
	provider *GitProvider
	config   *common.Config
	Repo     *gogit.Repository
	Worktree *gogit.Worktree
}

func (g *GitWorktree) CheckoutParent(depth int) error {
	hash, err := g.Repo.ResolveRevision(plumbing.Revision(fmt.Sprintf("HEAD~%v", depth)))

	if err != nil {
		return err
	}
	return g.Worktree.Checkout(&gogit.CheckoutOptions{
		Hash:   *hash,
		Create: false,
		Force:  true,
	})
}

func (g *GitWorktree) CheckoutHash(hash plumbing.Hash) error {
	return g.Worktree.Checkout(&gogit.CheckoutOptions{
		Hash:   hash,
		Create: false,
		Force:  true,
	})
}

func (g *GitWorktree) CheckoutBranch(branch plumbing.ReferenceName) error {
	return g.Worktree.Checkout(&gogit.CheckoutOptions{
		Branch: branch,
		Create: false,
		Force:  true,
	})
}

func (g *GitWorktree) ReadMetadata() (string, error) {
	file, err := g.Worktree.Filesystem.Open(MetadataFilename)

	if err != nil {
		return "", ErrMetadataRW
	}

	fileContents, err := ioutil.ReadAll(file)

	if err != nil {
		return "", ErrMetadataRW
	}

	return string(fileContents), nil
}

func (g *GitWorktree) WriteAndCommitMetadata(metadata string) (plumbing.Hash, error) {
	file, err := g.Worktree.Filesystem.Create(MetadataFilename)

	if err != nil {
		return plumbing.ZeroHash, ErrMetadataRW
	}

	if n, err := file.Write([]byte(metadata)); err != nil || n <= 0 {
		return plumbing.ZeroHash, ErrMetadataRW
	}

	if _, err := g.Worktree.Add(MetadataFilename); err != nil {
		return plumbing.ZeroHash, ErrMetadataCommit
	}

	return g.Worktree.Commit(common.ComputeHash(metadata), &gogit.CommitOptions{
		Author: &object.Signature{
			Name:  common.ComputeHash(g.config.SshKey)[:hashCutoffCount],
			Email: common.ComputeHash(g.config.SshKey)[:hashCutoffCount],
			When:  time.Now(),
		},
		Parents: []plumbing.Hash{}, // This is a no-op since the validator adds parents
	})
}
