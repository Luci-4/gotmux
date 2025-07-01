package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/Microsoft/go-winio"
)

const (
	inputPipe  = `\\.\pipe\gotmux_input`
	outputPipe = `\\.\pipe\gotmux_output`
)

func main() {
	out, err := winio.DialPipe(outputPipe, nil)
	if err != nil {
		fmt.Printf("Failed to connect to output pipe: %v\n", err)
		return
	}
	defer out.Close()

	in, err := winio.DialPipe(inputPipe, nil)
	if err != nil {
		fmt.Printf("Failed to connect to input pipe: %v\n", err)
		return
	}
	defer in.Close()

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

	reader := bufio.NewReader(os.Stdin)
	for {
		text, _ := reader.ReadString('\n')
		text = strings.TrimSpace(text)
		if text == "exit" {
			in.Write([]byte("exit\n"))
			break
		}
		in.Write([]byte(text + "\n"))
	}
}
