package git

import (
	"fmt"
	"sync"
	"time"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/storer"
	"github.com/go-git/go-git/v5/storage"
	"github.com/go-git/go-git/v5/storage/memory"
)

var ErrUnsupportedObjectType = fmt.Errorf("unsupported object type")

type StorageProvider struct {
	referenceStorage *SynchronizedReferenceStorage
	configStorage    *memory.ConfigStorage
	shallowStorage   *memory.ShallowStorage
	objectStorage    *SynchronizedObjectStorage
	moduleStorage    *memory.ModuleStorage
}

type ConcurrentStorage struct {
	memory.ConfigStorage
	SynchronizedObjectStorage
	memory.ShallowStorage
	SynchronizedReferenceStorage
	memory.ModuleStorage

	// The index storage should be unique across instances
	memory.IndexStorage
}

func NewStorageProvider() *StorageProvider {
	moduleStorage := make(memory.ModuleStorage)
	return &StorageProvider{
		referenceStorage: &SynchronizedReferenceStorage{
			lock: new(sync.Mutex),
			m:    make(memory.ReferenceStorage),
		},
		configStorage:  &memory.ConfigStorage{},
		shallowStorage: &memory.ShallowStorage{},
		objectStorage: &SynchronizedObjectStorage{
			lock:    new(sync.Mutex),
			Objects: make(map[plumbing.Hash]plumbing.EncodedObject),
			Commits: make(map[plumbing.Hash]plumbing.EncodedObject),
			Trees:   make(map[plumbing.Hash]plumbing.EncodedObject),
			Blobs:   make(map[plumbing.Hash]plumbing.EncodedObject),
			Tags:    make(map[plumbing.Hash]plumbing.EncodedObject),
		},
		moduleStorage: &moduleStorage,
	}
}

func (s *StorageProvider) NewSharedStorage() *ConcurrentStorage {
	return &ConcurrentStorage{
		SynchronizedReferenceStorage: *s.referenceStorage,
		ConfigStorage:                *s.configStorage,
		ShallowStorage:               *s.shallowStorage,
		SynchronizedObjectStorage:    *s.objectStorage,
		ModuleStorage:                *s.moduleStorage,
	}
}

type SynchronizedObjectStorage struct {
	// TODO change this lock type
	lock    *sync.Mutex
	Objects map[plumbing.Hash]plumbing.EncodedObject
	Commits map[plumbing.Hash]plumbing.EncodedObject
	Trees   map[plumbing.Hash]plumbing.EncodedObject
	Blobs   map[plumbing.Hash]plumbing.EncodedObject
	Tags    map[plumbing.Hash]plumbing.EncodedObject
}

func (o *SynchronizedObjectStorage) NewEncodedObject() plumbing.EncodedObject {
	return &plumbing.MemoryObject{}
}

func (o *SynchronizedObjectStorage) SetEncodedObject(obj plumbing.EncodedObject) (plumbing.Hash, error) {
	o.lock.Lock()
	defer o.lock.Unlock()

	h := obj.Hash()
	o.Objects[h] = obj

	switch obj.Type() {
	case plumbing.CommitObject:
		o.Commits[h] = o.Objects[h]
	case plumbing.TreeObject:
		o.Trees[h] = o.Objects[h]
	case plumbing.BlobObject:
		o.Blobs[h] = o.Objects[h]
	case plumbing.TagObject:
		o.Tags[h] = o.Objects[h]
	default:
		return h, ErrUnsupportedObjectType
	}

	return h, nil
}

func (o *SynchronizedObjectStorage) HasEncodedObject(h plumbing.Hash) (err error) {
	o.lock.Lock()
	defer o.lock.Unlock()

	if _, ok := o.Objects[h]; !ok {
		return plumbing.ErrObjectNotFound
	}
	return nil
}

func (o *SynchronizedObjectStorage) EncodedObjectSize(h plumbing.Hash) (
	size int64, err error) {
	o.lock.Lock()
	defer o.lock.Unlock()

	obj, ok := o.Objects[h]
	if !ok {
		return 0, plumbing.ErrObjectNotFound
	}

	return obj.Size(), nil
}

func (o *SynchronizedObjectStorage) EncodedObject(t plumbing.ObjectType, h plumbing.Hash) (plumbing.EncodedObject, error) {
	o.lock.Lock()
	defer o.lock.Unlock()

	obj, ok := o.Objects[h]
	if !ok || (plumbing.AnyObject != t && obj.Type() != t) {
		return nil, plumbing.ErrObjectNotFound
	}

	return obj, nil
}

func (o *SynchronizedObjectStorage) IterEncodedObjects(t plumbing.ObjectType) (storer.EncodedObjectIter, error) {
	o.lock.Lock()
	defer o.lock.Unlock()

	var series []plumbing.EncodedObject
	switch t {
	case plumbing.AnyObject:
		series = flattenObjectMap(o.Objects)
	case plumbing.CommitObject:
		series = flattenObjectMap(o.Commits)
	case plumbing.TreeObject:
		series = flattenObjectMap(o.Trees)
	case plumbing.BlobObject:
		series = flattenObjectMap(o.Blobs)
	case plumbing.TagObject:
		series = flattenObjectMap(o.Tags)
	}

	return storer.NewEncodedObjectSliceIter(series), nil
}

func flattenObjectMap(m map[plumbing.Hash]plumbing.EncodedObject) []plumbing.EncodedObject {
	objects := make([]plumbing.EncodedObject, 0, len(m))
	for _, obj := range m {
		objects = append(objects, obj)
	}
	return objects
}

func (o *SynchronizedObjectStorage) Begin() storer.Transaction {
	return &TxObjectStorage{
		Storage: o,
		Objects: make(map[plumbing.Hash]plumbing.EncodedObject),
	}
}

func (o *SynchronizedObjectStorage) ForEachObjectHash(fun func(plumbing.Hash) error) error {
	o.lock.Lock()
	defer o.lock.Unlock()

	for h := range o.Objects {
		err := fun(h)
		if err != nil {
			if err == storer.ErrStop {
				return nil
			}
			return err
		}
	}
	return nil
}

func (o *SynchronizedObjectStorage) ObjectPacks() ([]plumbing.Hash, error) {
	return nil, nil
}
func (o *SynchronizedObjectStorage) DeleteOldObjectPackAndIndex(plumbing.Hash, time.Time) error {
	return nil
}

var errNotSupported = fmt.Errorf("not supported")

func (o *SynchronizedObjectStorage) LooseObjectTime(hash plumbing.Hash) (time.Time, error) {
	return time.Time{}, errNotSupported
}
func (o *SynchronizedObjectStorage) DeleteLooseObject(plumbing.Hash) error {
	return errNotSupported
}

type TxObjectStorage struct {
	Storage *SynchronizedObjectStorage
	Objects map[plumbing.Hash]plumbing.EncodedObject
}

func (tx *TxObjectStorage) SetEncodedObject(obj plumbing.EncodedObject) (plumbing.Hash, error) {
	h := obj.Hash()
	tx.Objects[h] = obj

	return h, nil
}

func (tx *TxObjectStorage) EncodedObject(t plumbing.ObjectType, h plumbing.Hash) (plumbing.EncodedObject, error) {
	obj, ok := tx.Objects[h]
	if !ok || (plumbing.AnyObject != t && obj.Type() != t) {
		return nil, plumbing.ErrObjectNotFound
	}

	return obj, nil
}

func (tx *TxObjectStorage) Commit() error {
	for h, obj := range tx.Objects {
		delete(tx.Objects, h)
		if _, err := tx.Storage.SetEncodedObject(obj); err != nil {
			return err
		}
	}

	return nil
}

func (tx *TxObjectStorage) Rollback() error {
	tx.Objects = make(map[plumbing.Hash]plumbing.EncodedObject)
	return nil
}

type SynchronizedReferenceStorage struct {
	lock *sync.Mutex
	m    map[plumbing.ReferenceName]*plumbing.Reference
}

func (r *SynchronizedReferenceStorage) SetReference(ref *plumbing.Reference) error {
	// TODO the reason we need to put a fat mutex on the worktree operations is because
	// when we checkout, we modify the shared reference storage, affecting the HEAD reference
	// This means that without some sort of copy-on-write map for the read-only worktrees,
	// we cannot faithfully support concurrent operations. This can be a venture for the
	// future if that itch ever arises, but since the event is over, I'm not going to bother.
	r.lock.Lock()
	defer r.lock.Unlock()
	if ref != nil {
		r.m[ref.Name()] = ref
	}

	return nil
}

func (r *SynchronizedReferenceStorage) CheckAndSetReference(ref, old *plumbing.Reference) error {
	r.lock.Lock()
	defer r.lock.Unlock()
	if ref == nil {
		return nil
	}

	if old != nil {
		tmp := r.m[ref.Name()]
		if tmp != nil && tmp.Hash() != old.Hash() {
			return storage.ErrReferenceHasChanged
		}
	}
	r.m[ref.Name()] = ref
	return nil
}

func (r *SynchronizedReferenceStorage) Reference(n plumbing.ReferenceName) (*plumbing.Reference, error) {
	r.lock.Lock()
	defer r.lock.Unlock()
	ref, ok := r.m[n]
	if !ok {
		return nil, plumbing.ErrReferenceNotFound
	}

	return ref, nil
}

func (r *SynchronizedReferenceStorage) IterReferences() (storer.ReferenceIter, error) {
	r.lock.Lock()
	defer r.lock.Unlock()
	var refs []*plumbing.Reference
	for _, ref := range r.m {
		refs = append(refs, ref)
	}

	return storer.NewReferenceSliceIter(refs), nil
}

func (r *SynchronizedReferenceStorage) CountLooseRefs() (int, error) {
	return len(r.m), nil
}

func (r *SynchronizedReferenceStorage) PackRefs() error {
	return nil
}

func (r *SynchronizedReferenceStorage) RemoveReference(n plumbing.ReferenceName) error {
	r.lock.Lock()
	defer r.lock.Unlock()
	delete(r.m, n)
	return nil
}
