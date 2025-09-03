package engine

import (
	"errors"

	"github.com/vishalkuo/bimap"
)

var ErrKeyNotFound = errors.New("key not found")
var ErrValueNotFound = errors.New("value not found")

type KeyType = string
type ValueType = string

const DefaultValue = ""

type Cache struct {
	storage *bimap.BiMap[KeyType, ValueType]
}

func New() *Cache {
	return &Cache{storage: bimap.NewBiMap[KeyType, ValueType]()}
}

func (c *Cache) AddKeyValue(key KeyType, val ValueType) ValueType {
	presentValue, err := c.getValueForKey(key)
	if err != nil {
		c.storage.Insert(key, val)
		return val
	}

	return presentValue
}

func (c *Cache) getValueForKey(key KeyType) (ValueType, error) {
	if val, ok := c.storage.Get(key); ok {
		return val, nil
	}
	return DefaultValue, ErrValueNotFound
}

func (c *Cache) GetKeyForValue(value ValueType) (KeyType, error) {
	if val, ok := c.storage.GetInverse(value); ok {
		return val, nil
	}
	return DefaultValue, ErrKeyNotFound
}
