package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSet_Basic(t *testing.T) {
	assert := assert.New(t)
	
	// Create a new set
	set := NewSet[string]()
	assert.Equal(0, set.Len(), "New set should be empty")
	
	// Add elements
	set.Add("apple")
	set.Add("banana")
	set.Add("apple") // duplicate, should not affect size
	
	assert.Equal(2, set.Len(), "Set should have 2 elements")
	assert.True(set.Contains("apple"), "Set should contain 'apple'")
	assert.True(set.Contains("banana"), "Set should contain 'banana'")
	assert.False(set.Contains("orange"), "Set should not contain 'orange'")
	
	// Remove element
	set.Remove("apple")
	assert.Equal(1, set.Len(), "Set should have 1 element after removal")
	assert.False(set.Contains("apple"), "Set should not contain 'apple' after removal")
	
	// Clear
	set.Clear()
	assert.Equal(0, set.Len(), "Set should be empty after clearing")
}

func TestSet_FromSlice(t *testing.T) {
	assert := assert.New(t)
	
	slice := []int{1, 2, 3, 2, 1} // Contains duplicates
	set := NewSetFromSlice(slice)
	
	assert.Equal(3, set.Len(), "Set should have 3 unique elements")
	assert.True(set.Contains(1), "Set should contain 1")
	assert.True(set.Contains(2), "Set should contain 2")
	assert.True(set.Contains(3), "Set should contain 3")
}

func TestSet_Values(t *testing.T) {
	assert := assert.New(t)
	
	set := NewSet[string]()
	set.AddValues([]string{"apple", "banana", "cherry"})
	
	values := set.Values()
	assert.Len(values, 3, "Values should return a slice with 3 items")
	assert.Contains(values, "apple")
	assert.Contains(values, "banana")
	assert.Contains(values, "cherry")
}

func TestSet_SortedValues(t *testing.T) {
	assert := assert.New(t)
	
	set := NewSet[int]()
	set.AddValues([]int{3, 1, 4, 1, 5, 9, 2, 6, 5})
	
	sorted := SortedValues(set, func(a, b int) bool {
		return a < b
	})
	
	assert.Equal([]int{1, 2, 3, 4, 5, 6, 9}, sorted, "Values should be sorted in ascending order")
}

func TestSet_Union(t *testing.T) {
	assert := assert.New(t)
	
	set1 := NewSetFromSlice([]string{"apple", "banana", "cherry"})
	set2 := NewSetFromSlice([]string{"banana", "cherry", "date"})
	
	union := set1.Union(set2)
	
	assert.Equal(4, union.Len(), "Union should have 4 unique elements")
	assert.True(union.Contains("apple"), "Union should contain 'apple'")
	assert.True(union.Contains("banana"), "Union should contain 'banana'")
	assert.True(union.Contains("cherry"), "Union should contain 'cherry'")
	assert.True(union.Contains("date"), "Union should contain 'date'")
}

func TestSet_Intersection(t *testing.T) {
	assert := assert.New(t)
	
	set1 := NewSetFromSlice([]string{"apple", "banana", "cherry"})
	set2 := NewSetFromSlice([]string{"banana", "cherry", "date"})
	
	intersection := set1.Intersection(set2)
	
	assert.Equal(2, intersection.Len(), "Intersection should have 2 elements")
	assert.False(intersection.Contains("apple"), "Intersection should not contain 'apple'")
	assert.True(intersection.Contains("banana"), "Intersection should contain 'banana'")
	assert.True(intersection.Contains("cherry"), "Intersection should contain 'cherry'")
	assert.False(intersection.Contains("date"), "Intersection should not contain 'date'")
}

func TestSet_Difference(t *testing.T) {
	assert := assert.New(t)
	
	set1 := NewSetFromSlice([]string{"apple", "banana", "cherry"})
	set2 := NewSetFromSlice([]string{"banana", "cherry", "date"})
	
	diff := set1.Difference(set2)
	
	assert.Equal(1, diff.Len(), "Difference should have 1 element")
	assert.True(diff.Contains("apple"), "Difference should contain 'apple'")
	assert.False(diff.Contains("banana"), "Difference should not contain 'banana'")
	
	diff2 := set2.Difference(set1)
	assert.Equal(1, diff2.Len(), "Difference should have 1 element")
	assert.True(diff2.Contains("date"), "Difference should contain 'date'")
}