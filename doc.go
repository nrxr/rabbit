/*
Using rabbit package with publisher and consumer

Setting up a publisher

    srv, err := server.New(
        server.Address(rabbitaddr),
    )
    if err != nil { // handle error }

    pubtpl := amqp.Publishing{
        ContentType: "text/plain",
    }
    exch := exchange.New("stocks")
    pub, err := publisher.New(
        publisher.Server(srv),
        publisher.Exchange(exch),
        publisher.PublishingTemplate(pubtpl),
        publisher.RoutingKey("us.nyse.amd"),
    )

    err = pub.Send([]byte("message"))
    if err != nil { // handle error }

Setting up a consumer

To set up a consumer you need to declare a exchange, a queue and bind that queue
to a exchange. Optionally you can set Qos values for setting the prefetch size.

 srv, _ := server.New(server.Address(rabbitserver))
 xch := exchange.New("stocks")
 q := queue.New(
	 quque.Name("us.nyse.amd"),
	 queue.AddBinding(queue.Binding{Key. "us.nyse.amd"}),
	 queue.Durable(),
 )
 cns, _ := consumer.New(
	 consumer.Server(srv),
	 consumer.Exchange(xch),
	 consumer.Queue(q),
	 consumer.Qos(1),
 )

 amqclosed := srv.NotifyClose()
 go func() {
	 msgs := con.Subscribe()
	 for srv.Loop() {
		 select {
		 case msg := <-msgs:
			 // Do things...
			 msg.Ack(false)
		 }
	 }
 }()

 select {
 case <-amqclosed:
	 // closed, do something
 }

*/
package rabbit
