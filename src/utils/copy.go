package utils

// Copy a pointers value. Only applies for the first layer of data. Nested pointers are not copied.
func ShallowCopy[T any](data *T) *T {
	out := *data
	return &out
}
