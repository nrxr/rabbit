package main

import (
	"fmt"
	"time"

	"github.com/nrxr/rabbit/consumer"
	"github.com/nrxr/rabbit/exchange"
	"github.com/nrxr/rabbit/publisher"
	"github.com/nrxr/rabbit/queue"
	"github.com/nrxr/rabbit/server"
)

func send(p *publisher.Publisher, secs int) {
	time.Sleep(time.Second * time.Duration(secs))

	fmt.Println("trying to send things")

	err := p.Send([]byte("test"))
	if err != nil {
		panic(err)
	}
}

func shutserver(s *server.Server, secs int) {
	time.Sleep(time.Second * time.Duration(secs))

	s.Shutdown("reasons")
}

func main() {
	srv, err := server.New()
	if err != nil {
		panic(err)
	}

	xch := exchange.New("stocks")
	q := queue.New(
		queue.Name("us.nyse.amd"),
		queue.AddBinding(queue.Binding{Key: "us.nyse.amd"}),
		queue.AutoDelete(),
	)

	pub, err := publisher.New(
		publisher.Server(srv),
		publisher.Exchange(xch),
		publisher.RoutingKey("us.nyse.amd"),
	)
	if err != nil {
		panic(err)
	}

	go send(pub, 2)
	go send(pub, 3)
	go send(pub, 8)
	go shutserver(srv, 15)

	cns, err := consumer.New(
		consumer.Server(srv),
		consumer.Exchange(xch),
		consumer.Queue(q),
	)
	if err != nil {
		panic(err)
	}

	go func() {
		msgs := cns.Subscribe()
		fmt.Println("subscribed to queue")
		for srv.Loop() {
			fmt.Println("listening incoming messages")
			select {
			case m := <-msgs:
				fmt.Println("received message!")
				fmt.Println(string(m.Body))
				m.Ack(false)
			}
		}
	}()

	amqclosed := srv.NotifyClose()
	fmt.Println(<-amqclosed)
}
