package event

import (
	"reflect"
	"testing"
)

func TestUnmarshal(t *testing.T) {
	ev := UserConnected{
		UserID: "123",
	}

	p, err := newPayload(ev)
	if err != nil {
		t.Fatal(err)
	}

	u, _, err := newUnmarshaler(UserConnected{})
	if err != nil {
		t.Fatal(err)
	}

	ev2, err := u(p)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(ev, ev2) {
		t.Fatalf("not equal, want = %+v, actual = %+v", ev, ev2)
	}
}
