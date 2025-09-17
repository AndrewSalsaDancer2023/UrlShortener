package engine

type RandomGenerator interface {
	GenerateRandomValue() (uint64, error)
}
