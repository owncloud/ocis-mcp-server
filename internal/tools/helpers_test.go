package tools

import (
	"testing"
)

func TestBoolPtr(t *testing.T) {
	truePtr := boolPtr(true)
	falsePtr := boolPtr(false)

	if *truePtr != true {
		t.Error("expected *truePtr == true")
	}
	if *falsePtr != false {
		t.Error("expected *falsePtr == false")
	}
}

func TestReadOnlyAnnotations(t *testing.T) {
	a := readOnlyAnnotations()
	if !a.ReadOnlyHint {
		t.Error("expected ReadOnlyHint=true")
	}
	if a.DestructiveHint == nil || *a.DestructiveHint {
		t.Error("expected DestructiveHint=false")
	}
	if a.OpenWorldHint == nil || !*a.OpenWorldHint {
		t.Error("expected OpenWorldHint=true")
	}
}

func TestMutatingAnnotations(t *testing.T) {
	a := mutatingAnnotations()
	if a.ReadOnlyHint {
		t.Error("expected ReadOnlyHint=false")
	}
	if a.DestructiveHint == nil || *a.DestructiveHint {
		t.Error("expected DestructiveHint=false")
	}
}

func TestIdempotentAnnotations(t *testing.T) {
	a := idempotentAnnotations()
	if !a.IdempotentHint {
		t.Error("expected IdempotentHint=true")
	}
	if a.DestructiveHint == nil || *a.DestructiveHint {
		t.Error("expected DestructiveHint=false")
	}
}

func TestDestructiveAnnotations(t *testing.T) {
	a := destructiveAnnotations()
	if a.DestructiveHint == nil || !*a.DestructiveHint {
		t.Error("expected DestructiveHint=true")
	}
}
