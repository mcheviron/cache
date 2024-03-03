package cache

import (
	"reflect"
	"testing"
)

func TestNewItem(t *testing.T) {
	// Test case 1: Integer value
	intValue := 42
	item1 := newItem("key", intValue, 0)
	expectedSize1 := int(reflect.TypeOf(intValue).Size())

	if item1.size != expectedSize1 {
		t.Errorf("Expected item size to be %d, got %d", expectedSize1, item1.size)
	}

	// Test case 2: Struct value
	type myStruct struct {
		Name   string
		Number int
	}
	structValue := myStruct{Name: "John", Number: 123}
	item2 := newItem("key", structValue, 0)
	expectedSize2 := int(reflect.TypeOf(structValue).Size())
	println(expectedSize2)

	if item2.size != expectedSize2 {
		t.Errorf("Expected item size to be %d, got %d", expectedSize2, item2.size)
	}
}
