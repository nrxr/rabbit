package server

import (
	"testing"
)

func TestNew(t *testing.T) {
	addr := "amqp://guest:guest@fakelocalhost:5672/"
	r, err := New(Address(addr))
	if err == nil {
		t.Errorf("should not return a nil error")
	}

	if r.addr != addr {
		t.Errorf("addr returned %s and expected %s", r.addr, addr)
	}

	if r.IsOpen() {
		t.Errorf("Rabbit.IsOpen() returned true, it should return false since it's closed")
	}

	ch, err := r.Channel()
	if ch != nil {
		t.Errorf("Rabbit.Channel() should return a nil channel")
	}

	if err == nil {
		t.Errorf("Rabbit.Channel()'s error should not be nil")
	}

	if err.Error() != ENOCONN {
		t.Errorf("Rabbit.Channel() expected to be %s and got %s", ENOCONN, err.Error())
	}
}
