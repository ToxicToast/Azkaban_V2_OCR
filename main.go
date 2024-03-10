package main

import (
	"encoding/base64"
	"encoding/json"
	"github.com/otiai10/gosseract/v2"
	amqp "github.com/rabbitmq/amqp091-go"
	"log"
	"os"
)

type MqttResponse struct {
	Pattern string `json:"pattern"`
	Data    string `json:"data"`
	ID      string `json:"id"`
}

func failOnError(err error, msg string) {
	if err != nil {
		log.Fatalf("%s: %s", msg, err)
	}
}

func processImage(data []byte) string {

	// Create a temporary file to save the image for processing
	tmpfile, err := os.CreateTemp("", "temp-image-*.png")
	failOnError(err, "Can not create temporary file")
	defer os.Remove(tmpfile.Name()) // clean up

	if _, err := tmpfile.Write(data); err != nil {
		tmpfile.Close()
		log.Fatal(err)
	}
	if err := tmpfile.Close(); err != nil {
		log.Fatal(err)
	}

	// Initialize the client
	client := gosseract.NewClient()
	defer client.Close()

	// Set the image to the temporary file path
	client.SetImage(tmpfile.Name())

	// Set Language
	client.SetLanguage("deu")

	// Perform OCR on the image file
	text, err := client.Text()
	if err != nil {
		log.Fatal(err)
	}

	return text
}

func main() {
	username := os.Getenv("BROKER_USERNAME")
	password := os.Getenv("BROKER_PASSWORD")
	hostname := os.Getenv("BROKER_HOST")
	port := os.Getenv("BROKER_PORT")

	conn, err := amqp.Dial("amqp://" + username + ":" + password + "@" + hostname + ":" + port + "/")
	failOnError(err, "Failed to connect to RabbitMQ")
	defer conn.Close()

	ch, err := conn.Channel()
	failOnError(err, "Failed to open a channel")
	defer ch.Close()

	q, err := ch.QueueDeclare(
		"inventory_receipe_queue", // name
		true,                      // durable
		false,                     // delete when unused
		false,                     // exclusive
		false,                     // no-wait
		nil,                       // arguments
	)
	failOnError(err, "Failed to declare a queue")

	msgs, err := ch.Consume(
		q.Name, // queue
		"",     // consumer
		true,   // auto-acknowledge
		false,  // exclusive
		false,  // no-local
		false,  // no-wait
		nil,    // args
	)
	failOnError(err, "Failed to register a consumer")

	forever := make(chan bool)

	go func() {
		for d := range msgs {
			var mqttResponse MqttResponse
			err := json.Unmarshal(d.Body, &mqttResponse)
			failOnError(err, "Failed to unmarshal json")
			byteData, err := base64.StdEncoding.DecodeString(mqttResponse.Data)
			failOnError(err, "Failed to decode base64 to bytedata")
			text := processImage(byteData)
			//
			topicName := q.Name + "_ack"
			responseData := transformResponse(mqttResponse.ID, text)
			err = ch.Publish(
				"",        // Exchange
				topicName, // Routing key (Queue name)
				false,     // Mandatory
				false,     // Immediate
				amqp.Publishing{ // Message
					CorrelationId: mqttResponse.ID,
					MessageId:     mqttResponse.ID,
					ContentType:   "application/json",
					Body:          responseData,
				})
			failOnError(err, "Failed to publish response")
			log.Println("Received Text:", text)
		}
	}()

	log.Printf(" [*] Waiting for messages. To exit press CTRL+C")
	<-forever
}

type MqttRequest struct {
	ID   string `json:"id"`
	Text string `json:"text"`
}

func transformResponse(id string, text string) []byte {
	var response MqttRequest
	response.ID = id
	response.Text = text
	responseData, err := json.Marshal(response)
	failOnError(err, "Failed to marshal response data")
	return responseData
}
