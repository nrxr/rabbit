package queue

import "github.com/streadway/amqp"

// Queue hold definition of AMQP queue
type Queue struct {
	Name       string
	Durable    bool
	AutoDelete bool
	Exclusive  bool
	NoWait     bool
	Args       amqp.Table

	Bindings []Binding
}

// Binding used to declare binding between AMQP Queue and AMQP Exchange
type Binding struct {
	Key    string
	NoWait bool
	Args   amqp.Table
}

type Option func(Queue) Queue

func AddBinding(b Binding) Option {
	return func(q Queue) Queue {
		q.Bindings = append(q.Bindings, b)
		return q
	}
}

func Name(name string) Option {
	return func(q Queue) Queue {
		q.Name = name
		return q
	}
}

func Durable() Option {
	return func(q Queue) Queue {
		q.Durable = true
		return q
	}
}

func AutoDelete() Option {
	return func(q Queue) Queue {
		q.AutoDelete = true
		return q
	}
}

func Exclusive() Option {
	return func(q Queue) Queue {
		q.Exclusive = true
		return q
	}
}

func NoWait() Option {
	return func(q Queue) Queue {
		q.NoWait = true
		return q
	}
}

func Args(a amqp.Table) Option {
	return func(q Queue) Queue {
		q.Args = a
		return q
	}
}

/*
New returns a Queue definition with the following defaults unless changed via
the Option options.

 Queue{
	 Name: "",
	 Durable: false,
	 AutoDelete: false,
	 Exclusive: false,
	 NoWait: false,
	 Args: nil,
	 Bindings: []Binding{},
 }

*/
func New(opts ...Option) Queue {
	q := Queue{}

	for _, opt := range opts {
		q = opt(q)
	}

	return q
}
