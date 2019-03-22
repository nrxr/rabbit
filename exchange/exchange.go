/*
Package exchange takes care of RabbitMQ Exchange declarations. If the type Exchange were to be used directly it would end-up like:

 Exchange{
   Name: "",
   Kind: "",
   Durable: false,
   AutoDelete: false,
   Internal: false,
   NoWait: false,
   Args: nil,
 }

Becase of that, the New method allows better sensible options, so it would be this:

 Exchange{
   Name: "",
   Kind: "direct",
   Durable: true,
   AutoDelete: false,
   Internal: false,
   NoWait: false,
   Args: nil,
 }
*/
package exchange

import "github.com/streadway/amqp"

// Exchange holds the definition of an AMQP exchange.
type Exchange struct {
	Name string

	// Possible options are direct, fanout, topic and headers.
	Kind string

	Durable    bool
	AutoDelete bool
	Internal   bool
	NoWait     bool
	Args       amqp.Table
}

type Option func(Exchange) Exchange

// Kind changes the kind of an Exchange from the "direct" default.
func Kind(k string) Option {
	return func(e Exchange) Exchange {
		e.Kind = k
		return e
	}
}

// NoDurable changes the durability of an Exchange from the true default.
func NoDurable() Option {
	return func(e Exchange) Exchange {
		e.Durable = false
		return e
	}
}

func AutoDelete() Option {
	return func(e Exchange) Exchange {
		e.AutoDelete = true
		return e
	}
}

func Internal() Option {
	return func(e Exchange) Exchange {
		e.Internal = true
		return e
	}
}

func NoWait() Option {
	return func(e Exchange) Exchange {
		e.NoWait = true
		return e
	}
}

func Args(v amqp.Table) Option {
	return func(e Exchange) Exchange {
		e.Args = v
		return e
	}
}

/*
New returns an Exchange definition with the following defaults unless
changed via the Option options.

 Exchange{
   Name: "passedname",
   Kind: "direct",
   Durable: true,
   AutoDelete: false,
   Internal: false,
   NoWait: false,
   Args: nil,
 }

*/
func New(name string, opts ...Option) Exchange {
	e := Exchange{
		Name:    name,
		Kind:    "direct",
		Durable: true,
	}

	for _, opt := range opts {
		e = opt(e)
	}

	return e
}
