package engine

import (
	"errors"
	"sync"

	"github.com/vishalkuo/bimap"
)

var ErrKeyNotFound = errors.New("Key not found")
var ErrValueNotFound = errors.New("Value not found")

type KeyType = string
type ValueType = string

const DefaultValue = ""

type Cache struct {
	sync.RWMutex
	storage *bimap.BiMap[KeyType, ValueType]
}

func New() *Cache {
	return &Cache{storage: bimap.NewBiMap[KeyType, ValueType]()}
}

func (c *Cache) AddKeyValue(key KeyType, val ValueType) {
	c.Lock()
	defer c.Unlock()
	c.storage.Insert(key, val)
}

func (c *Cache) GetValueForKey(key KeyType) (ValueType, error) {
	c.RLock()
	defer c.RUnlock()
	if val, ok := c.storage.Get(key); ok {
		return val, nil
	}

	return DefaultValue, ErrValueNotFound
}

func (c *Cache) GetKeyForValue(value ValueType) (KeyType, error) {
	c.RLock()
	defer c.RUnlock()
	if val, ok := c.storage.GetInverse(value); ok {
		return val, nil
	}

	return DefaultValue, ErrKeyNotFound
}
