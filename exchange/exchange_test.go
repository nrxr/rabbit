package exchange

import "testing"

func TestNew(t *testing.T) {
	name := "test1749763019837"
	t.Run("plain New", func(t *testing.T) {
		e := New(name)
		if e.Name != name {
			t.Errorf(
				"New(\"%s\"): Name value should be %s and instead it has %s",
				name,
				name,
				e.Name,
			)
		}
	})

	t.Run("different kind", func(t *testing.T) {
		e := New(name, Kind("topic"))
		if e.Kind != "topic" {
			t.Errorf(
				"New: Kind value should be topic and instead it has %s",
				e.Kind,
			)
		}
	})

	t.Run("full run", func(t *testing.T) {
		e := New(
			name,
			Kind("fanout"),
			AutoDelete(),
			Internal(),
		)

		if !e.AutoDelete || !e.Internal {
			t.Errorf(
				"New: Needed autodelete and internal as true, instead got\n%v",
				e,
			)
		}
	})
}
