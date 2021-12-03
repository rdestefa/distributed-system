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

const LOGFILENAME = "backend.log"
const ADDRESS = "0.0.0.0:10000"

func main() {
	// Run main server loop and handle any unexpected errors
	err := run()
	if err != nil {
		log.Println(err)
		panic(err)
	}
}

func run() error {
	// Listen to address
	l, err := net.Listen("tcp", ADDRESS)
	if err != nil {
		return err
	}
	fmt.Printf("Listening on http://%v\n", l.Addr())

	// Setup log file
	// logFile, err := os.OpenFile(LOGFILENAME, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0666)
	// if err != nil {
	// 	return err
	// }
	// defer logFile.Close()
	// log.SetOutput(logFile)

	// Setup http server and connect to address
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

	// Handle signals
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt)
	select {
	case err := <-errc:
		log.Printf("Failed to serve: %v\n", err)
	case sig := <-sigs:
		log.Printf("Terminating: %v\n", sig)
	}

	// Upon signal, wait 10 seconds and force shutdown
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	return hs.Shutdown(ctx)
}
