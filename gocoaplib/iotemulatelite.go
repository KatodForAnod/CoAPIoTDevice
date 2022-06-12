package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"github.com/plgd-dev/go-coap/v2"
	"github.com/plgd-dev/go-coap/v2/message"
	"github.com/plgd-dev/go-coap/v2/message/codes"
	"github.com/plgd-dev/go-coap/v2/mux"
	"log"
	"time"
)

type infType string

const (
	timeType = "timeType"
	tickType = "tickType"
)

type iotExample struct {
	switcher infType
}

func (receiver *iotExample) loggingMiddleware(next mux.Handler) mux.Handler {
	return mux.HandlerFunc(func(w mux.ResponseWriter, r *mux.Message) {
		log.Printf("ClientAddress %v, %v\n", w.Client().RemoteAddr(), r.String())
		next.ServeCOAP(w, r)
	})
}

func (receiver *iotExample) handleTimeSwitch(w mux.ResponseWriter, r *mux.Message) {
	if !r.IsConfirmable {
		return
	}

	receiver.switcher = timeType

	err := w.SetResponse(codes.Content, message.AppJSON, bytes.NewReader([]byte("{\"status\":\"ok\"}")))
	if err != nil {
		log.Printf("cannot set response: %v", err)
	}
}

func (receiver *iotExample) handleTickSwitch(w mux.ResponseWriter, r *mux.Message) {
	if !r.IsConfirmable {
		return
	}

	receiver.switcher = tickType

	err := w.SetResponse(codes.Content, message.AppJSON, bytes.NewReader([]byte("{\"status\":\"ok\"}")))
	if err != nil {
		log.Printf("cannot set response: %v", err)
	}
}

func (receiver *iotExample) observInf(w mux.ResponseWriter, r *mux.Message) {
	log.Printf("Got message path=%v: %+v from %v", receiver.getPath(r.Options), r, w.Client().RemoteAddr())
	obs, err := r.Options.Observe()
	switch {
	case r.Code == codes.GET && err == nil && obs == 0:
		go receiver.periodicTransmitter(w.Client(), r.Token)
	case r.Code == codes.GET:
		subded := time.Now()
		err := receiver.sendResponse(w.Client(), r.Token, subded, -1)
		if err != nil {
			log.Printf("Error on transmitter: %v", err)
		}
	}
}

func (receiver *iotExample) getPath(opts message.Options) string {
	path, err := opts.Path()
	if err != nil {
		log.Printf("cannot get path: %v", err)
		return ""
	}
	return path
}

func (receiver *iotExample) periodicTransmitter(cc mux.Client, token []byte) {
	subded := time.Now()

	for obs := int64(0); ; obs++ {
		err := receiver.sendResponse(cc, token, subded, obs)
		if err != nil {
			log.Printf("Error on transmitter, stopping: %v", err)
			return
		}
		time.Sleep(time.Second)
	}
}

func (receiver *iotExample) sendResponse(cc mux.Client, token []byte, subded time.Time, obs int64) error {
	m := message.Message{
		Code:    codes.Content,
		Token:   token,
		Context: cc.Context(),
		Body:    bytes.NewReader([]byte(fmt.Sprintf("Hello World"))),
	}

	if receiver.switcher == timeType {
		m.Body = bytes.NewReader([]byte(fmt.Sprintf("Been running for %v", time.Since(subded))))
	} else if receiver.switcher == tickType {
		m.Body = bytes.NewReader([]byte(fmt.Sprintf("Been running for %v", obs)))
	}

	var opts message.Options
	buf := make([]byte, 256)
	opts, n, err := opts.SetContentFormat(buf, message.TextPlain)
	if errors.Is(err, message.ErrTooSmall) {
		buf = append(buf, make([]byte, n)...)
		opts, _, err = opts.SetContentFormat(buf, message.TextPlain)
	}
	if err != nil {
		return fmt.Errorf("cannot set content format to response: %w", err)
	}

	buf = buf[n:]
	if obs >= 0 {
		opts, n, err = opts.SetObserve(buf, uint32(obs))
		if errors.Is(err, message.ErrTooSmall) {
			buf = append(buf, make([]byte, n)...)
			opts, _, err = opts.SetObserve(buf, uint32(obs))
		}
		if err != nil {
			return fmt.Errorf("cannot set options to response: %w", err)
		}
	}

	m.Options = opts
	return cc.WriteMessage(&m)
}

func main() {
	var port string
	var timeB bool
	flag.StringVar(&port, "port",
		"5688", "port of iot server")
	flag.BoolVar(&timeB, "time", false, "default tick sup, u can turn on time sup")
	flag.Parse()

	example := iotExample{}
	if timeB {
		example.switcher = timeType
	} else {
		example.switcher = tickType
	}

	r := mux.NewRouter()
	r.Use(example.loggingMiddleware)
	r.Handle("/tick", mux.HandlerFunc(example.handleTickSwitch))
	r.Handle("/time", mux.HandlerFunc(example.handleTimeSwitch))
	r.Handle("/some/path", mux.HandlerFunc(example.observInf))

	log.Fatal(coap.ListenAndServe("udp", ":"+port, r))
}
