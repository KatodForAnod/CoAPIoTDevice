package main

import (
	"github.com/zubairhamed/canopus"
)

func handlerA(req canopus.Request) canopus.Response {
	msg := canopus.ContentMessage(req.GetMessage().GetMessageId(), canopus.MessageAcknowledgment)
	msg.SetStringPayload("Acknowledged: " + req.GetMessage().GetPayload().String())

	res := canopus.NewResponse(msg, nil)
	return res
}

func main() {
	server := canopus.NewServer()

	server.Get("/hello", handlerA)

	server.ListenAndServe(":5683")
}
