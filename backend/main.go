package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"time"
)

const LOGFILENAME = "backend.log"
const ADDRESS = "0.0.0.0:10000"

var (
	WarnLogger  *log.Logger
	InfoLogger  *log.Logger
	ErrorLogger *log.Logger
)

func init() {
	logLevel := strings.ToLower(os.Getenv("LOGLEVEL"))
	if logLevel == "" {
		logLevel = "error"
	}

	InfoLogger = log.New(ioutil.Discard, "", 0)
	WarnLogger = log.New(ioutil.Discard, "", 0)
	ErrorLogger = log.New(ioutil.Discard, "", 0)

	switch logLevel {
	case "info":
		InfoLogger = log.New(os.Stderr, "[INFO]  ", log.Ldate|log.Ltime|log.Lshortfile)
		fallthrough
	case "warn":
		WarnLogger = log.New(os.Stderr, "[WARN]  ", log.Ldate|log.Ltime|log.Lshortfile)
		fallthrough
	case "":
		fallthrough
	case "error":
		ErrorLogger = log.New(os.Stderr, "[ERROR] ", log.Ldate|log.Ltime|log.Lshortfile)
	default:
		// no logging in case the user passes some other value like "none"
	}
}

func main() {
	// Run main server loop and handle any unexpected errors
	err := run()
	if err != nil {
		ErrorLogger.Println(err)
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
		ErrorLogger.Printf("Failed to serve: %v\n", err)
	case sig := <-sigs:
		WarnLogger.Printf("Terminating: %v\n", sig)
	}

	// Upon signal, wait 10 seconds and force shutdown
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	return hs.Shutdown(ctx)
}
