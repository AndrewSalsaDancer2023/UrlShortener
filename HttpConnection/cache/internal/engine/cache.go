package engine

import (
	"errors"

	"github.com/vishalkuo/bimap"
	"urlshortener.com/cache"
)

var ErrKeyNotFound = errors.New("key not found")
var ErrValueNotFound = errors.New("value not found")

const DefaultValue = ""

type Cache struct {
	storage *bimap.BiMap[cache.KeyType, cache.ValueType]
}

func New() *Cache {
	return &Cache{storage: bimap.NewBiMap[cache.KeyType, cache.ValueType]()}
}

func (c *Cache) AddKeyValue(key cache.KeyType, val cache.ValueType) cache.ValueType {
	presentValue, err := c.GetValueForKey(key)
	if err != nil {
		c.storage.Insert(key, val)
		return val
	}

	return presentValue
}

func (c *Cache) GetValueForKey(key cache.KeyType) (cache.ValueType, error) {
	if val, ok := c.storage.Get(key); ok {
		return val, nil
	}
	return DefaultValue, ErrValueNotFound
}

func (c *Cache) GetKeyForValue(value cache.ValueType) (cache.KeyType, error) {
	if val, ok := c.storage.GetInverse(value); ok {
		return val, nil
	}
	return DefaultValue, ErrKeyNotFound
}
