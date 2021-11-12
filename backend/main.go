package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"time"
)

const ADDRESS = "0.0.0.0:9000"

func main() {
	err := run()
	if err != nil {
		fmt.Println(err)
	}
}

func run() error {
	l, err := net.Listen("tcp", ADDRESS)
	if err != nil {
		return err
	}
	fmt.Printf("Listening on http://%v", l.Addr())

	s := newServer()
	hs := &http.Server{
		Handler:      s,
		ReadTimeout:  time.Second * 10,
		WriteTimeout: time.Second * 10,
	}
	errc := make(chan error, 1)
	go func() {
		errc <- hs.Serve(l)
	}()

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt)
	select {
	case err := <-errc:
		log.Printf("Failed to serve: %v\n", err)
	case sig := <-sigs:
		log.Printf("Terminating: %v\n", sig)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	return hs.Shutdown(ctx)
}
