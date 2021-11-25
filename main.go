package main

import (
	"bufio"
	"flag"
	"github.com/gorilla/websocket"
	"log"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"time"
)

var addr = flag.String("addr", "localhost:8080", "http service address")
var handlerRoute = flag.String("path", "/echo", "path where the websocket handler listens")
var cmdFlag = flag.String("cmd", "ping -n 10 google.com", "command to log")


var (
	InfoLogger  *log.Logger
	ErrorLogger *log.Logger
)

func init() {
	InfoLogger = log.New(os.Stdout, "[i] ", log.Ldate|log.Ltime)
	ErrorLogger = log.New(os.Stderr, "[X] ", log.Ldate|log.Ltime)
}

func commandScanner(cmdString string) (chan string, error) {

	cmdParts := strings.Split(cmdString, " ")

	command := exec.Command(cmdParts[0], cmdParts[1:]...)

	commandOut, _ := command.StdoutPipe()
	err := command.Start()
	if err != nil {
		return nil, err
	}

	scanner := bufio.NewScanner(commandOut)
	scanner.Split(bufio.ScanLines)

	commandOutChan := make(chan string, 1)

	go func() {
		defer close(commandOutChan)
		for scanner.Scan() {
			commandOutChan <- scanner.Text()
		}

		err = command.Wait()
		if err != nil {
			log.Fatalln(err)
		}
	}()

	return commandOutChan, nil
}

func main() {
	flag.Parse()
	log.SetFlags(0)
	InfoLogger.Printf("Executing command \"%s\"", *cmdFlag)
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	u := url.URL{Scheme: "ws", Host: *addr, Path: *handlerRoute}

	InfoLogger.Printf("Connecting to: %s", u.String())
	c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		ErrorLogger.Fatalln(err.Error())
	}

	defer func(c *websocket.Conn) {
		_ = c.Close()
	}(c)

	done := make(chan struct{})

	go func() {
		defer close(done)
		for {
			_, message, err := c.ReadMessage()
			if err != nil {
				return
			}

			InfoLogger.Println(string(message))
		}
	}()


	cmdOut, err := commandScanner(*cmdFlag)

	if err != nil {
		ErrorLogger.Println(err.Error())
		return
	}

	for cmdLine := range cmdOut {
		err := c.WriteMessage(websocket.TextMessage, []byte(cmdLine))
		if err != nil {
			ErrorLogger.Println(err.Error())
			break
		}
	}

	InfoLogger.Println("Stopping...")

	err = c.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))

	if err != nil {
		ErrorLogger.Println(err.Error())
	}

	select {
	case <-done:
	case <-time.After(time.Second):
	}

}
