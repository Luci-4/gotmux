package main

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"os"
	"time"

	"golang.org/x/term"
	"github.com/Microsoft/go-winio"
)

const (
	inputPipe   = `\\.\pipe\win_tmux_input`
	outputPipe  = `\\.\pipe\win_tmux_output`
	controlPipe = `\\.\pipe\win_tmux_control`
)

func dialPipeWithRetry(pipeName string, maxRetries int, delay time.Duration) (net.Conn, error) {
	var conn net.Conn
	var err error
	for i := 0; i < maxRetries; i++ {
		conn, err = winio.DialPipe(pipeName, nil)
		if err == nil {
			return conn, nil
		}
		time.Sleep(delay)
	}
	return nil, err
}

func main() {
	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		log.Fatalf("failed to set terminal raw mode: %v", err)
	}
	defer term.Restore(int(os.Stdin.Fd()), oldState)

	ctrlConn, err := dialPipeWithRetry(controlPipe, 50, 100*time.Millisecond)
	if err != nil {
		fmt.Printf("Failed to connect to control pipe: %v\n", err)
		return
	}
	defer ctrlConn.Close()
	log.Println("Connected to control pipe")

	inConn, err := dialPipeWithRetry(inputPipe, 50, 100*time.Millisecond)
	if err != nil {
		fmt.Printf("Failed to connect to input pipe: %v\n", err)
		return
	}
	defer inConn.Close()
	log.Println("Connected to input pipe")

	outConn, err := dialPipeWithRetry(outputPipe, 50, 100*time.Millisecond)
	if err != nil {
		fmt.Printf("Failed to connect to output pipe: %v\n", err)
		return
	}
	defer outConn.Close()
	log.Println("Connected to output pipe")

	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := outConn.Read(buf)
			if err != nil {
				if err.Error() != "EOF" {
					fmt.Printf("Read error: %v\n", err)
				}
				break
			}
			if n > 0 {
				fmt.Print(string(buf[:n]))
			}
		}
	}()

	inReader := bufio.NewReader(os.Stdin)
	controlWriter := ctrlConn
	rawWriter := inConn

	for {
		b, err := inReader.ReadByte()
		if err != nil {
			break
		}

		if b == 7 { // Ctrl+G (ASCII 7)
			term.Restore(int(os.Stdin.Fd()), oldState)

			fmt.Print("\nControl command mode. Enter command: ")
			lineReader := bufio.NewReader(os.Stdin)
			cmdLine, err := lineReader.ReadString('\n')
			if err != nil {
				fmt.Println("Error reading command:", err)
			} else {
				controlWriter.Write([]byte(cmdLine))
			}

			oldState, err = term.MakeRaw(int(os.Stdin.Fd()))
			if err != nil {
				panic(err)
			}
			continue
		}

		_, err = rawWriter.Write([]byte{b})
		if err != nil {
			break
		}
	}
}
