// Package publisher makes handling RabbitMQ's publishing tasks easier.
package publisher

import (
	"fmt"
	"sync"

	"github.com/nrxr/rabbit/exchange"
	"github.com/nrxr/rabbit/server"

	"github.com/streadway/amqp"
)

// A Publisher represents a RabbitMQ publisher of messages against an specific
// queue. It implements logic to have always a connection available before
// trying to send a message and reconnect as possible.
type Publisher struct {
	server   *server.Server
	exchange exchange.Exchange
	channel  *amqp.Channel

	routingKey string
	tpl        amqp.Publishing
	mandatory  bool
	immediate  bool

	m sync.Mutex
}

// ENOSERVER is returned when the server.Server passed is incorrect, nil or
// there was no server.Server passed. Check the method Server() under the
// Option type.
const ENOSERVER = "incorrect or no server passed. Use Rabbit() to pass one"

// ENOEXCHANGE is returned when no server.Exchange is properly declared. The
// default is not enough, it needs a name.
const ENOEXCHANGE = "no exchange declared correctly"

type Option func(*Publisher) error

// Server adds a server.Server representation to a Publisher.
func Server(r *server.Server) Option {
	return func(pub *Publisher) error {
		if r == nil {
			return fmt.Errorf(ENOSERVER)
		}
		pub.server = r

		return nil
	}
}

// RoutingKey adds a Routing Key value during the publishing action of the
// Publisher.
func RoutingKey(k string) Option {
	return func(pub *Publisher) error {
		pub.routingKey = k
		return nil
	}
}

// PublishingTemplate adds a template of the amqp.Publishing structure to the
// Publisher. If this is not set the only default value in the Publishing
// template is ContentType as "text/plain".
func PublishingTemplate(tpl amqp.Publishing) Option {
	return func(pub *Publisher) error {
		pub.tpl = tpl
		return nil
	}
}

// Set the mandatory option in the Publish action. Defaults to false, the only
// logic use of Mandatory() method is to set it true.
func Mandatory() Option {
	return func(pub *Publisher) error {
		pub.mandatory = true
		return nil
	}
}

// Set the immediate option in the Publish action. Defaults to false, the only
// logic use of Immediate() method is to set it true.
func Immediate() Option {
	return func(pub *Publisher) error {
		pub.immediate = true
		return nil
	}
}

// Exchange adds a server.Exchange definition to a Publisher.
func Exchange(e exchange.Exchange) Option {
	return func(pub *Publisher) error {
		pub.exchange = e

		return nil
	}
}

// New returns a Publisher or an error if is not in a working state. A Server
// argument is required, if it's missing it will return a server.ENOCONN and if
// it's incorrect it will return a ENOSERVER. If no Exchange is passed it will
// return a ENOEXCHANGE.
func New(opts ...Option) (*Publisher, error) {
	p := &Publisher{}

	for _, opt := range opts {
		err := opt(p)
		if err != nil {
			return nil, err
		}
	}

	if err := p.validate(); err != nil {
		return nil, err
	}

	if err := p.init(); err != nil {
		return nil, err
	}

	return p, nil
}

// init returns an error if it's not possible to initialize a *amqp.Channel
// from the server or something fails during the Exchange declaration.
func (p *Publisher) init() error {
	p.m.Lock()
	defer p.m.Unlock()

	if p.channel != nil {
		p.channel.Close()
	}

	var err error
	p.channel, err = p.server.Channel()
	if err != nil {
		return err
	}

	if p.tpl.ContentType == "" {
		p.tpl.ContentType = "text/plain"
	}

	err = p.channel.ExchangeDeclare(
		p.exchange.Name,
		p.exchange.Kind,
		p.exchange.Durable,
		p.exchange.AutoDelete,
		p.exchange.Internal,
		p.exchange.NoWait,
		p.exchange.Args,
	)
	if err != nil {
		return err
	}

	return nil
}

// validate returns an error if the server is not initialized or the exchange
// lacks at least a name.
func (p *Publisher) validate() error {
	if p.server == nil {
		return fmt.Errorf(ENOSERVER)
	}

	if p.exchange.Name == "" {
		return fmt.Errorf(ENOEXCHANGE)
	}

	return nil
}

// RoutingKey returns the routing key to which is publishing against the
// exchange.
func (p *Publisher) RoutingKey() string {
	return p.routingKey
}

// Send returns an error if something goes wrong while publishing. If it's
// awaiting for a reconnection then it will block until either a reconnection
// succeeds or a close notification is sent, in which case it will return the
// closing error and it won't try to send the message. On subsequent calls it
// will return a ENOSERVER becase server.Server.Loop() in that case will be
// false.
func (p *Publisher) Send(body []byte) error {
	pub := p.tpl
	pub.Body = body

	// If this is false it will block until either the reconnection attempt is
	// good or the connection get's closed in which case we will return a
	// ENOSERVER.
	if !p.server.IsOpen() {
		if !p.server.Loop() {
			return fmt.Errorf(ENOSERVER)
		}

		reconn := p.server.NotifyReconnect()
		closer := p.server.NotifyClose()

		select {
		case err := <-closer:
			return err
		case <-reconn:
			if err := p.init(); err != nil {
				return err
			}
		}
	}

	return p.channel.Publish(
		p.exchange.Name,
		p.routingKey,
		p.mandatory,
		p.immediate,
		pub,
	)
}

// Close will call the method Close against the *amqp.Channel.
func (p *Publisher) Close() {
	p.channel.Close()
}
