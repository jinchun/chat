package Rabbit

import (
	"github.com/streadway/amqp"
	"log"
)

type Rabbit struct {
	Conn *amqp.Connection
	Ch   *amqp.Channel
	Q    amqp.Queue
}

func (r *Rabbit) failOnError(err error, msg string) {
	if err != nil {
		log.Fatalf("%s: %s", msg, err)
	}
}

func (r *Rabbit) Con() {
	conn, err := amqp.Dial("amqp://guest:guest@localhost:32773/")

	r.failOnError(err, "Failed to connect to RabbitMQ")
	//defer conn.Close()
	r.Conn = conn
	r.Ch = r.channel()
	r.Q = r.queue()
}
func (r *Rabbit) queue() amqp.Queue {
	q, err := r.Ch.QueueDeclare(
		"default", // name
		false,     // durable
		false,     // delete when unused
		false,     // exclusive
		false,     // no-wait
		nil,       // arguments
	)
	r.failOnError(err, "Failed to declare a queue")
	return q
}
func (r *Rabbit) Pub(body string) {
	err := r.Ch.Publish(
		"",       // exchange
		r.Q.Name, // routing key
		false,    // mandatory
		false,    // immediate
		amqp.Publishing{
			//ContentType: "text/plain",
			Body: []byte(body),
		})
	log.Printf(" [x] Sent %s", body)
	r.failOnError(err, "Failed to publish a message")
}

func (r *Rabbit) channel() *amqp.Channel {
	ch, err := r.Conn.Channel()
	r.failOnError(err, "Failed to open a channel")
	return ch
}

func (r *Rabbit) Consume() <-chan amqp.Delivery {
	msgs, err := r.Ch.Consume(
		r.Q.Name, // queue
		"",       // consumer
		true,     // auto-ack
		false,    // exclusive
		false,    // no-local
		false,    // no-wait
		nil,      // args
	)
	r.failOnError(err, "Failed to register a consumer")
	return msgs
}
