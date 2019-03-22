/*
Package server consolidates all the methods and functions required to handle
connections and reconnections against RabbitMQ servers.

Usage

The way to use it is by calling the New method. This will generate a Server
that will take care of all the errors and reconnection handling. Then from
the *amqp.Connection pointer returned by Server.Get() you can start
exchanges and subscribe or publish to queues.

 r, err := server.New(
   server.Address("amqp://user:pass@host.tld:port"),
 )
 if err != nil {  // handle error }

 reconn := r.NotifyReconnect()
 closer := r.NotifyClose()
 errors := r.NotifyErrors()

 for r.Loop() {
   select {
   case err := <-closer:
     // The connection was closed definitively, nothing else will be sent here or anything.
	 //	If it was closed because of an error, the err it will not be nil. Handle the error.
	 if err != nil {
	   // do something with the error here...
	 }
   case <-reconn:
     // There was a reconnect, re-declare channels and queues so it can use the new connection.
   case err := <-errors:
     // Do something with the error or something...
   }
 }

The way rabbit can die is by either a reconnection error or because
Server.Shutdown was called. It will notify via the Server.NotifyClose channel
of type error. If it's nil it means it was closed cleanly.

In the implementation you can use Server.IsOpen() to know if the connection is
open or not, if it's not you should not try to send messages as a publisher.
*/
package server

import (
	"fmt"
	"log"
	"sync"
	"sync/atomic"

	"github.com/nrxr/maskpass"

	"github.com/streadway/amqp"
)

const ENOCONN = "no connection available"

// A Server represents a RabbitMQ connection and it's states.
type Server struct {
	conn atomic.Value // *amqp.Connection
	addr string

	reconnAttempt int32
	reconnLimit   int32

	closes     []chan error
	errors     []chan error
	reconnects []chan bool

	// open communicates the current state of the connection. It's supposed to
	// be modified only by using atomic operations on it with 0 meaning false
	// and 1 meaning true as in standard binary. When the connection suceeds it
	// will be 1, when there's an error or any NotifyClose event it will be 0
	// until a reconnection succeeds and then moves back to 1.
	open int32

	// Log is an optional value
	log *log.Logger

	mux sync.Mutex
}

// An Option represents an optional value that can be passed to the New
// constructor of Server.
type Option func(*Server) error

// Address receives an URL string value passed for the server connection string
// of Rabbit's. The default value that will be used is
// amqp://guest:guest@localhost:5672/. For the URI specification used by
// RabbitMQ check https://www.rabbitmq.com/uri-spec.html .
func Address(addr string) Option {
	return func(r *Server) error {
		if addr == "" {
			return fmt.Errorf("rabbit/server: Address passed is empty")
		}

		r.addr = addr

		return nil
	}
}

// ReconnectionLimits receives an integer of how many times to attempt to
// reconnect. Default value is 5.
func ReconnectionLimits(limit int32) Option {
	return func(r *Server) error {
		if limit < 0 {
			return fmt.Errorf("rabbit/server: reconnection limit value passed is negative")
		}

		r.reconnLimit = limit

		return nil
	}
}

// Logger receives a standard log.Logger interface to use as a logger.
func Logger(l *log.Logger) Option {
	return func(r *Server) error {
		r.log = l
		return nil
	}
}

// New returns an initialized and connected *Server using the environment
// variables as values. If a logger is passed it will send as informational
// messages the reconnections and shutdowns.
func New(opts ...Option) (*Server, error) {
	r := &Server{
		// Defaults
		addr:        "amqp://guest:guest@localhost:5672/",
		reconnLimit: 5,
	}

	for _, opt := range opts {
		err := opt(r)
		if err != nil {
			return r, err
		}
	}

	if err := r.connect(); err != nil {
		return r, err
	}

	return r, nil
}

// notifyHandler takes care of looping over the notify channel and calling the
// reconnect method.
func (r *Server) notifyHandler(notify chan *amqp.Error) {
	for {
		select {
		case err := <-notify:
			if r.log != nil {
				r.log.Printf("received NotifyClose with error value %v\n", err)
			}

			// If the error is nil that means r.conn.Close was called so we
			// should not try to reconnect.
			//
			// The reconnection will only be called if r.open is not already 0,
			// if it's already 0 then reconnect is very likely already running.
			//
			// As an extra precaution this section is locked with a sync.Mutex
			// so no other call would run it.
			r.mux.Lock()
			if err != nil && atomic.LoadInt32(&r.open) == 1 {
				go r.sendErrors(err)
				r.reconnect()
			}
			atomic.StoreInt32(&r.open, 0)
			r.mux.Unlock()

			break
		}
	}
}

// connect returns an error if the connection process had any issue during the
// amqp.Dial call.
func (r *Server) connect() error {
	conn, err := amqp.Dial(r.addr)
	if err != nil {
		go r.sendErrors(err)
		return err
	}

	r.conn.Store(conn)
	atomic.StoreInt32(&r.open, 1)

	notify := conn.NotifyClose(make(chan *amqp.Error))
	go r.notifyHandler(notify)

	return nil
}

// reconnect will handle all the reconnection logic by receiving as argument
// the *amqp.Error value handed by the *amqp.Connection.NotifyClose method. If
// the argument passed is nil it won't try to reconnect since it will assume
// the nil was sent because *amqp.Connection.Close was called.
func (r *Server) reconnect() {
	atomic.AddInt32(&r.reconnAttempt, 1)

	if atomic.LoadInt32(&r.reconnAttempt) > r.reconnLimit {
		r.close(fmt.Errorf(
			"rabbit: can't reconnect to server %s, tried %d times",
			maskpass.String(r.addr),
			r.reconnLimit,
		))
		return
	}

	if err := r.connect(); err != nil {
		if r.log != nil {
			r.log.Printf(
				`couldn't reconnect to RabbitMQ server.
Address:	%s
Attempt:	%d
Error:
%v`,
				maskpass.String(r.addr),
				atomic.LoadInt32(&r.reconnAttempt),
				err,
			)
		}

		go r.sendErrors(err)
		r.reconnect()
	}

	for _, ch := range r.reconnects {
		ch <- true
	}

	atomic.StoreInt32(&r.reconnAttempt, 0)
}

// connection returns an *amqp.Connection pointer that can be passed to other
// methods and take advantage of the reconnection handling implemented by the
// rabbit package.
func (r *Server) connection() (*amqp.Connection, error) {
	conn, _ := r.conn.Load().(*amqp.Connection)
	if conn == nil {
		return nil, fmt.Errorf(ENOCONN)
	}
	return conn, nil
}

// Channel returns an *amqp.Channel that can be used to declare exchanges and
// publish/consume from queue.
func (r *Server) Channel() (*amqp.Channel, error) {
	conn, err := r.connection()
	if err != nil {
		return nil, err
	}

	return conn.Channel()
}

// Shutdown returns an error if the shutdown process of the RabbitMQ connection
// returned an error. A reason is expected and it will be shown in the logs if
// *Server.log is not nil.
func (r *Server) Shutdown(reason string) error {
	if r.log != nil {
		r.log.Printf(
			"shutting down RabbitMQ connection to %s. Reason: %s",
			maskpass.String(r.addr),
			reason,
		)
	}
	return r.close(nil)
}

// sendErrors is a handy wrapper for sending errors to all the error channels
// opened up.
func (r *Server) sendErrors(err error) {
	for _, ch := range r.errors {
		ch <- err
	}
}

// close returns an error if Server.conn.Close() method goes wrong in any way.
// This method will be called by either the Server.reconnect() method in case
// it can't reconnect after the limit of retries or if Server.Shutdown() was
// called externally.
func (r *Server) close(err error) error {
	r.mux.Lock()
	defer r.mux.Unlock()

	atomic.StoreInt32(&r.open, 0)

	// closing the close channels after notifying of the closure.
	for _, ch := range r.closes {
		ch <- err
		close(ch)
	}

	conn, err := r.connection()
	if err != nil {
		return err
	}

	cerr := conn.Close()
	if cerr != nil {
		return cerr
	}

	// closing the other channels.
	for _, ch := range r.reconnects {
		close(ch)
	}
	for _, ch := range r.errors {
		close(ch)
	}

	r.conn.Store((*amqp.Connection)(nil))

	return nil
}

// IsOpen returns a boolean that communicates the current state of the
// connection against the RabbitMQ server. As soon as the connection is
// established with a Server.connect() call it will be return true. During a
// amqp.NotifyClose or Server.Shutdown it will be set to false. During a
// reconnect it will be false until the reconnect succeeds.
//
// If you want to be sure the connection is open, check with Server.Open before
// making any operation against the RabbitMQ server.
func (r *Server) IsOpen() bool {
	if atomic.LoadInt32(&r.open) == 1 {
		return true
	}

	return false
}

// Loop returns true if the connection is open or there's an active attempt to
// reconnect and get the Server to a working condition. This is specially
// useful to keep for-loops listening to the channels generated by Server as
// long as they actually exist.
func (r *Server) Loop() bool {
	if r.IsOpen() {
		return true
	}

	if atomic.LoadInt32(&r.reconnAttempt) <= r.reconnLimit {
		return true
	}

	return false
}

// NotifyClose returns a receiving-only channel with the error interface that
// will be only be called once, if the RabbitMQ connection is closed. The error
// returned will be nil if the close was created by the Server.Shutdown()
// method, else it will return an error.
func (r *Server) NotifyClose() <-chan error {
	r.mux.Lock()
	defer r.mux.Unlock()
	ch := make(chan error)
	r.closes = append(r.closes, ch)
	return ch
}

// NotifyReconnect returns a receiving-only channel with true when a reconnect
// succeeds. In case of a reconnection event, the channels and queues linked to
// the Server connection need to be rebuilt.
func (r *Server) NotifyReconnect() <-chan bool {
	r.mux.Lock()
	defer r.mux.Unlock()

	ch := make(chan bool)
	r.reconnects = append(r.reconnects, ch)
	return ch
}

// NotifyErrors returns a receiving-only channel with the error interface that
// will be receive messages each time something throws an error within the
// Server representation. Receiving an error does not means something fatal
// happened. If something fatal happens the error will be received in a closing
// channel. (Check the NotifyClose's method documentation.)
func (r *Server) NotifyErrors() <-chan error {
	r.mux.Lock()
	defer r.mux.Unlock()

	ch := make(chan error)
	r.errors = append(r.errors, ch)
	return ch
}
