package cache

type KeyType = string
type ValueType = string

type Cacher interface {
	AddKeyValue(key KeyType, val ValueType) ValueType
	GetValueForKey(key KeyType) (ValueType, error)
	GetKeyForValue(value ValueType) (KeyType, error)
}
