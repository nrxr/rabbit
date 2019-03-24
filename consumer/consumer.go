/*
Package consumer consolidates all the functions and methods for queue
consumers. It implements a reconnection method as well.

Usage

 con, err := consumer.New(
	 consume.Server(server),
	 consumer.Exchange(exchange),
	 consume.Queue(queue),
 )
 for server.Loop() {
	 select {
	 case msg := <-con.Subscribe():
		 // do things with the message
	 }
 }
*/
package consumer

import (
	"fmt"
	"sync"

	"github.com/nrxr/rabbit/exchange"
	"github.com/nrxr/rabbit/queue"
	"github.com/nrxr/rabbit/server"

	"github.com/streadway/amqp"
)

// A Consumer represents a RabbitMQ consumer of messages against a set of
// queues. It implements logic to have always a connection available where to
// receive messages from.
type Consumer struct {
	server   *server.Server
	channel  *amqp.Channel
	exchange exchange.Exchange

	qpassed bool
	queue   queue.Queue
	dqueue  amqp.Queue

	qosCount int

	tag       string
	autoack   bool
	exclusive bool
	internal  bool
	nowait    bool
	args      amqp.Table

	listeners []chan amqp.Delivery
	messages  <-chan amqp.Delivery

	m sync.Mutex
}

// ENOSERVER is returned when there's no valid server.Server definition. It may
// mean an error on the connection or the initialization.
const ENOSERVER = "no valid server in rabbit"

const ENOEXCHANGE = "no exchange declared"

const ENOQUEUE = "no queue declared, can't consume without queues"

type Option func(*Consumer) error

// Server adds a Server definition to the Consumer as a server value.
func Server(r *server.Server) Option {
	return func(c *Consumer) error {
		if r == nil {
			return fmt.Errorf(ENOSERVER)
		}
		c.server = r

		return nil
	}
}

// Exchange adds a rabbit.Exchange definition to Consumer.
func Exchange(e exchange.Exchange) Option {
	return func(c *Consumer) error {
		c.exchange = e
		return nil
	}
}

// Queue adds a rabbit.Queue to a slice of rabbit.Queue's in the consumer.
func Queue(q queue.Queue) Option {
	return func(c *Consumer) error {
		c.queue = q
		c.qpassed = true
		return nil
	}
}

// Tag adds a consumer tag being sent as the consumer value.
func Tag(tag string) Option {
	return func(c *Consumer) error {
		c.tag = tag
		return nil
	}
}

// AutoAck makes the consumer auto-acknowledge messages as soon as they are
// received.
func AutoAck() Option {
	return func(c *Consumer) error {
		c.autoack = true
		return nil
	}
}

// Exclusive marks as true the exclusive value on the Consume method from AMQP.
func Exclusive() Option {
	return func(c *Consumer) error {
		c.exclusive = true
		return nil
	}
}

// Internal changes the boolean internal value to true.
func Internal() Option {
	return func(c *Consumer) error {
		c.internal = true
		return nil
	}
}

// NoWait changes the boolean nowait value to true.
func NoWait() Option {
	return func(c *Consumer) error {
		c.nowait = true
		return nil
	}
}

// Args changes the args to consume call.
func Args(a amqp.Table) Option {
	return func(c *Consumer) error {
		c.args = a
		return nil
	}
}

// Qos sets the prefetch value for the consuming of queues.
func Qos(q int) Option {
	return func(c *Consumer) error {
		c.qosCount = q
		return nil
	}
}

// New returns a Consumer or an error if it's not in a working state. A Server
// argument is required, if it's missing it will return a rabbit.ENOCONN and if
// it's an incorrect value it will return a ENOSERVER.
func New(opts ...Option) (*Consumer, error) {
	c := &Consumer{}

	for _, opt := range opts {
		if err := opt(c); err != nil {
			return nil, err
		}
	}

	if err := c.validate(); err != nil {
		return nil, err
	}

	if err := c.init(); err != nil {
		return nil, err
	}

	return c, nil
}

// consume starts the consume, sends it to the messages channel and inits the
// serve method as a goroutine.
func (c *Consumer) consume() error {
	var err error
	c.messages, err = c.channel.Consume(
		c.dqueue.Name,
		c.tag,
		c.autoack,
		c.exclusive,
		c.internal,
		c.nowait,
		c.args,
	)

	if err != nil {
		return err
	}

	go c.serve()

	return nil
}

func (c *Consumer) serve() {
	for c.server.Loop() {
		select {
		case d := <-c.messages:
			for _, ch := range c.listeners {
				ch <- d
			}
		}
	}
}

func (c *Consumer) Subscribe() <-chan amqp.Delivery {
	c.m.Lock()
	defer c.m.Unlock()
	ch := make(chan amqp.Delivery)

	c.listeners = append(c.listeners, ch)

	return ch
}

// init returns an error if any of the processes fail. The processes ran here
// are: channel creation, Exchange declaration, Queues declarations and Queue
// bindings declarations.
func (c *Consumer) init() error {
	c.m.Lock()
	defer c.m.Unlock()

	var err error

	if c.channel != nil {
		c.channel.Close()
	}

	c.channel, err = c.server.Channel()
	if err != nil {
		return err
	}

	if c.qosCount > 0 {
		err = c.channel.Qos(c.qosCount, 0, false)
		if err != nil {
			return err
		}
	}

	err = c.channel.ExchangeDeclare(
		c.exchange.Name,
		c.exchange.Kind,
		c.exchange.Durable,
		c.exchange.AutoDelete,
		c.exchange.Internal,
		c.exchange.NoWait,
		c.exchange.Args,
	)
	if err != nil {
		return err
	}

	c.dqueue, err = c.channel.QueueDeclare(
		c.queue.Name,
		c.queue.Durable,
		c.queue.AutoDelete,
		c.queue.Exclusive,
		c.queue.NoWait,
		c.queue.Args,
	)
	if err != nil {
		return err
	}

	for _, b := range c.queue.Bindings {
		err := c.channel.QueueBind(
			c.dqueue.Name,
			b.Key,
			c.exchange.Name,
			b.NoWait,
			b.Args,
		)

		if err != nil {
			return err
		}
	}

	if err := c.consume(); err != nil {
		return err
	}

	return nil
}

// validate returns an error if the server is not declared (ENOSERVER), the
// exchange is not declared (ENOEXCHANGE) or the Queue was not passed
// (ENOQUEUE).
func (c *Consumer) validate() error {
	if c.server == nil {
		return fmt.Errorf(ENOSERVER)
	}

	if c.exchange.Name == "" {
		return fmt.Errorf(ENOEXCHANGE)
	}

	if !c.qpassed {
		return fmt.Errorf(ENOQUEUE)
	}

	return nil
}

// RoutingKeys returns a slice of all the routing keys being used against the
// queue in this consumer.
func (c *Consumer) RoutingKeys() []string {
	rk := []string{}
	for _, b := range c.queue.Bindings {
		rk = append(rk, b.Key)
	}

	return rk
}
