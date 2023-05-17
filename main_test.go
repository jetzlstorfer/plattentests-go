package main

import (
	"reflect"
	"testing"

	"github.com/zmb3/spotify/v2"
)

func TestRemoveDuplicates(t *testing.T) {
	input := []spotify.ID{"1", "2", "3", "1", "2", "4"}
	expected := []spotify.ID{"1", "2", "3", "4"}

	output := removeDuplicates(input)

	if !reflect.DeepEqual(output, expected) {
		t.Errorf("removeDuplicates(%v) = %v; want %v", input, output, expected)
	}
}
