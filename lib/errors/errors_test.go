package errors

import (
	"testing"
)

func TestNewErrorWithMessage(t *testing.T) {
	e := NewErrorWithMessage(
		ErrNil, "failed",
	)
	v, ok := e.(Codes)
	if ok {
		t.Fatalf("ok%s  ", v.Message())
	} else {
		t.Fatalf("%s", v.Message())
	}

}
