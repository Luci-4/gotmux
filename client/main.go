package main

import (
	"bufio"
	"fmt"
	"os"
	"log"
	"golang.org/x/term"
	"github.com/Microsoft/go-winio"
)

const (
	inputPipe  = `\\.\pipe\gotmux_input`
	outputPipe = `\\.\pipe\gotmux_output`
	controlPipe = `\\.\pipe\gotmux_control`
)

func main() {
	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		log.Fatalf("failed to set terminal raw mode: %v", err)
	}
	defer term.Restore(int(os.Stdin.Fd()), oldState)

	ctrlConn, err := winio.DialPipe(controlPipe, nil)

	if err != nil {
		fmt.Printf("Failed to connect to control pipe: %v\n", err)
		return
	}
	defer ctrlConn.Close()
	log.Println("Connected to control pipe")

	out, err := winio.DialPipe(outputPipe, nil)
	if err != nil {
		fmt.Printf("Failed to connect to output pipe: %v\n", err)
		return
	}
	defer out.Close()

	log.Println("Connected to output pipe")
	inConn, err := winio.DialPipe(inputPipe, nil)
	if err != nil {
		fmt.Printf("Failed to connect to input pipe: %v\n", err)
		return
	}
	defer inConn.Close()

	log.Println("Connected to input pipe")

	go func() {
		buf := make([]byte, 4096) 
		for {
			n, err := out.Read(buf)
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

        if b == 7 { // Ctrl+G (ASCII 7), 
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
