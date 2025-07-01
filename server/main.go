package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"syscall"

	"github.com/charmbracelet/x/conpty"
	"github.com/Microsoft/go-winio"
)

const (
	inputPipeName  = `\\.\pipe\gotmux_input`
	outputPipeName = `\\.\pipe\gotmux_output`
)

var (
	currentPty    *conpty.ConPty
	currentHandle syscall.Handle
)

func createConPty(width, height int) (*conpty.ConPty, error) {
	pty, err := conpty.New(width, height, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to create ConPTY: %w", err)
	}
	return pty, nil
}

func setupChildProcess(pty *conpty.ConPty) *syscall.ProcAttr {
	return &syscall.ProcAttr{
		Files: []uintptr{
			pty.InPipeFd(),
			pty.OutPipeFd(),
			pty.OutPipeFd(),
		},
	}
}

func spawnProcess(pty *conpty.ConPty, cmd string, attr *syscall.ProcAttr) (int, uintptr, error) {
	pid, handle, err := pty.Spawn(cmd, []string{}, attr)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to spawn process: %w", err)
	}
	return pid, handle, nil
}

func startNamedPipeCommunication(pty *conpty.ConPty) {
	inListener, err := winio.ListenPipe(inputPipeName, nil)
	if err != nil {
		log.Fatalf("Failed to create input pipe: %v", err)
	}
	defer inListener.Close()

	outListener, err := winio.ListenPipe(outputPipeName, nil)
	if err != nil {
		log.Fatalf("Failed to create output pipe: %v", err)
	}
	defer outListener.Close()

	log.Println("Waiting for client to connect to output pipe...")
	outConn, err := outListener.Accept()
	if err != nil {
		log.Fatalf("Output pipe accept failed: %v", err)
	}
	defer outConn.Close()
	log.Println("Client connected to output pipe")

	log.Println("Waiting for client to connect to input pipe...")
	inConn, err := inListener.Accept()
	if err != nil {
		log.Fatalf("Input pipe accept failed: %v", err)
	}
	defer inConn.Close()
	log.Println("Client connected to input pipe")

	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := pty.Read(buf)
			if err != nil {
				if err != io.EOF {
					log.Printf("PTY read error: %v", err)
				}
				break
			}
			if n > 0 {

				_, err := outConn.Write(buf[:n])
				if err != nil {
					log.Printf("Failed to write to output pipe: %v", err)
					break
				}
			}
		}
	}()

	scanner := bufio.NewScanner(inConn)
	for scanner.Scan() {
		cmd := scanner.Text()
		if cmd == "exit" {
			log.Println("Received 'exit' command. Shutting down.")
			break
		}
		_, err := pty.Write([]byte(cmd + "\r"))
		if err != nil {
			log.Printf("Failed to write to PTY: %v", err)
			break
		}
	}
}

func main() {
	fmt.Println("Starting gotmux server...")

	pty, err := createConPty(80, 30)
	if err != nil {
		log.Fatalf("Failed to create PTY: %v", err)
	}
	defer pty.Close()
	currentPty = pty

	attr := setupChildProcess(pty)
	_, handle, err := spawnProcess(pty, `C:\Windows\System32\cmd.exe`, attr)
	if err != nil {
		log.Fatalf("Failed to start shell: %v", err)
	}
	currentHandle = syscall.Handle(handle)
	defer syscall.CloseHandle(currentHandle)

	log.Println("PTY session started. Awaiting pipe communication.")
	startNamedPipeCommunication(pty)

	log.Println("Session ended.")
}
