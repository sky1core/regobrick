package types

import (
	"encoding/json"
)

// RegoSet is a generic set for comparable elements. It is serialized as a JSON array,
// and duplicates are removed during unmarshaling.
type RegoSet[T comparable] map[T]struct{}

// MarshalJSON converts the set into a JSON array of unique elements.
func (rs RegoSet[T]) MarshalJSON() ([]byte, error) {
	arr := make([]T, 0, len(rs))
	for elem := range rs {
		arr = append(arr, elem)
	}
	return json.Marshal(arr)
}

// UnmarshalJSON reads a JSON array into the set, removing duplicates.
func (rs *RegoSet[T]) UnmarshalJSON(data []byte) error {
	var arr []T
	if err := json.Unmarshal(data, &arr); err != nil {
		return err
	}
	newSet := make(RegoSet[T], len(arr))
	for _, item := range arr {
		newSet[item] = struct{}{}
	}
	*rs = newSet
	return nil
}

// Add adds the specified element to the set.
func (rs RegoSet[T]) Add(elem T) {
	rs[elem] = struct{}{}
}

// Has returns true if the given element is in the set.
func (rs RegoSet[T]) Has(elem T) bool {
	_, ok := rs[elem]
	return ok
}
