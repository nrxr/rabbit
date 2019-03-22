package publisher

import (
	"testing"

	"github.com/nrxr/rabbit/exchange"
	"github.com/nrxr/rabbit/server"
)

func TestNew(t *testing.T) {
	t.Run("empty args", func(t *testing.T) {
		p, err := New()
		if err == nil {
			t.Errorf("New() should return error about missing Rabbit")
		}
		if p != nil {
			t.Errorf("New() should return nil Publisher")
		}
	})

	t.Run("nil Rabbit, ENOSERVER", func(t *testing.T) {
		_, err := New(Server(nil))
		if err == nil {
			t.Errorf("New(nilRabbit) should return error")
		}
		if err.Error() != ENOSERVER {
			t.Errorf("error should be %s and received %s", ENOSERVER, err.Error())
		}
	})

	t.Run("empty Rabbit, empty Exchange, ENOEXCHANGE", func(t *testing.T) {
		_, err := New(
			Server(&server.Server{}),
		)
		if err.Error() != ENOEXCHANGE {
			t.Errorf("error should be %s and received %s", ENOEXCHANGE, err.Error())
		}
	})

	t.Run("empty Rabbit, valid Exchange, ENOCONN", func(t *testing.T) {
		p, err := New(
			Server(&server.Server{}),
			Exchange(exchange.Exchange{Name: "test"}),
		)
		if err == nil {
			t.Errorf("New(emptyRabbit) should return error")
		}
		if err.Error() != server.ENOCONN {
			t.Errorf("error should be %s and received %s", server.ENOCONN, err.Error())
		}

		if p != nil {
			t.Errorf("New(emptyRabbit) should return nil Publisher")
		}
	})
}
