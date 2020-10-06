package Queue

import (
	"github.com/streadway/amqp"
	"log"
	"os"
)

type Rabbit struct {
}

func failOnError(err error, msg string) {
	if err != nil {
		log.Fatalf("%s: %s", msg, err)
	}
}

var conn *amqp.Connection
var channel *amqp.Channel
var queue amqp.Queue

func init() {
	_conn, err := amqp.Dial(os.Getenv("RABBITMQ_CONNECT"))

	failOnError(err, "Failed to connect to RabbitMQ")
	channel, err = _conn.Channel()
	failOnError(err, "Failed to open a channel")
	_queue, err := channel.QueueDeclare(
		"default", // name
		false,     // durable
		false,     // delete when unused
		false,     // exclusive
		false,     // no-wait
		nil,       // arguments
	)
	failOnError(err, "Failed to declare a queue")
	queue = _queue
	conn = _conn

	log.Println("rabbitmq init...")
}

func (r *Rabbit) Publish(body string) {
	err := channel.Publish(
		"",         // exchange
		queue.Name, // routing key
		false,      // mandatory
		false,      // immediate
		amqp.Publishing{
			//ContentType: "text/plain",
			Body: []byte(body),
		})
	log.Printf("[send]:%s", body)
	failOnError(err, "Failed to publish a message")
}

func (r *Rabbit) Consume() <-chan amqp.Delivery {
	deliveries, err := channel.Consume(
		queue.Name, // queue
		"",         // consumer
		true,       // auto-ack
		false,      // exclusive
		false,      // no-local
		false,      // no-wait
		nil,        // args
	)
	failOnError(err, "Failed to register a consumer")

	return deliveries
}

func (r *Rabbit) Close() {
	if err := conn.Close(); err != nil {
		log.Println("rabbitmq close failed.", err.Error())

		return
	}
	log.Println("rabbitmq closed.")
}
