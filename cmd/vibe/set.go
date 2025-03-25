package main

import (
	"sort"
)

// Set is a generic set data structure that stores unique values of type T
type Set[T comparable] struct {
	items map[T]struct{}
}

// NewSet creates a new empty set
func NewSet[T comparable]() *Set[T] {
	return &Set[T]{
		items: make(map[T]struct{}),
	}
}

// NewSetFromSlice creates a new set with values from the given slice
func NewSetFromSlice[T comparable](values []T) *Set[T] {
	set := NewSet[T]()
	set.AddValues(values)
	return set
}

// Add adds a value to the set
func (s *Set[T]) Add(value T) {
	s.items[value] = struct{}{}
}

// AddValues adds multiple values to the set
func (s *Set[T]) AddValues(values []T) {
	for _, value := range values {
		s.Add(value)
	}
}

// Remove removes a value from the set
func (s *Set[T]) Remove(value T) {
	delete(s.items, value)
}

// Contains checks if the set contains a value
func (s *Set[T]) Contains(value T) bool {
	_, exists := s.items[value]
	return exists
}

// Len returns the number of elements in the set
func (s *Set[T]) Len() int {
	return len(s.items)
}

// Clear removes all elements from the set
func (s *Set[T]) Clear() {
	s.items = make(map[T]struct{})
}

// Values returns a slice containing all values in the set (in no particular order)
func (s *Set[T]) Values() []T {
	values := make([]T, 0, len(s.items))
	for value := range s.items {
		values = append(values, value)
	}
	return values
}

// SortedValues returns a sorted slice of all values in the set
// This only works for types that can be compared with < operator
func SortedValues[T comparable](s *Set[T], less func(a, b T) bool) []T {
	values := s.Values()
	sort.Slice(values, func(i, j int) bool {
		return less(values[i], values[j])
	})
	return values
}

// Union returns a new set with all elements from both sets
func (s *Set[T]) Union(other *Set[T]) *Set[T] {
	result := NewSet[T]()
	
	// Add all elements from this set
	for value := range s.items {
		result.Add(value)
	}
	
	// Add all elements from the other set
	for value := range other.items {
		result.Add(value)
	}
	
	return result
}

// Intersection returns a new set with elements that exist in both sets
func (s *Set[T]) Intersection(other *Set[T]) *Set[T] {
	result := NewSet[T]()
	
	// Use the smaller set for iteration to optimize
	if s.Len() <= other.Len() {
		for value := range s.items {
			if other.Contains(value) {
				result.Add(value)
			}
		}
	} else {
		for value := range other.items {
			if s.Contains(value) {
				result.Add(value)
			}
		}
	}
	
	return result
}

// Difference returns a new set with elements in this set that are not in the other set
func (s *Set[T]) Difference(other *Set[T]) *Set[T] {
	result := NewSet[T]()
	
	for value := range s.items {
		if !other.Contains(value) {
			result.Add(value)
		}
	}
	
	return result
}