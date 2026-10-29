package profiles

import (
	"slices"
	"sync"
	"time"

	"github.com/google/uuid"
	"golang.org/x/sync/singleflight"
)

// namespaceTTL is how long a good answer of the namespace list is reused.
const namespaceTTL = 30 * time.Second

// namespaceCache shares one run of the namespace list among concurrent calls
// for a revision, and keeps a good answer for namespaceTTL. Its zero value is
// ready.
type namespaceCache struct {
	group   singleflight.Group
	mu      sync.Mutex
	entries map[uuid.UUID]namespaceEntry
}

type namespaceEntry struct {
	list    NamespaceList
	expires time.Time
}

// get returns the kept answer of revision at now.
func (c *namespaceCache) get(revision uuid.UUID, now time.Time) (NamespaceList, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.entries[revision]
	if !ok || !now.Before(entry.expires) {
		return NamespaceList{}, false
	}
	return copyOf(entry.list), true
}

// keep stores a good answer of revision at now and drops the expired ones.
func (c *namespaceCache) keep(revision uuid.UUID, list NamespaceList, now time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for key, entry := range c.entries {
		if !now.Before(entry.expires) {
			delete(c.entries, key)
		}
	}
	if c.entries == nil {
		c.entries = map[uuid.UUID]namespaceEntry{}
	}
	c.entries[revision] = namespaceEntry{list: copyOf(list), expires: now.Add(namespaceTTL)}
}

// load answers from the kept answer, or runs run once for all concurrent
// callers of revision and keeps its answer when its code is CodeOK.
func (c *namespaceCache) load(revision uuid.UUID, now func() time.Time, run func() (NamespaceList, error)) (NamespaceList, error) {
	if list, ok := c.get(revision, now()); ok {
		return list, nil
	}
	value, err, _ := c.group.Do(revision.String(), func() (any, error) {
		list, err := run()
		if err == nil && list.Code == CodeOK {
			c.keep(revision, list, now())
		}
		return list, err
	})
	if err != nil {
		return NamespaceList{}, err
	}
	return copyOf(value.(NamespaceList)), nil
}

func copyOf(list NamespaceList) NamespaceList {
	list.Namespaces = slices.Clone(list.Namespaces)
	return list
}
