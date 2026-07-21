package router

import (
	"reflect"
	"testing"
)

func TestSplitOrigins(t *testing.T) {
	want := []string{"http://localhost:5173", "http://127.0.0.1:5173"}
	got := splitOrigins(" http://localhost:5173, http://127.0.0.1:5173 ")
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("splitOrigins() = %#v, want %#v", got, want)
	}
}
