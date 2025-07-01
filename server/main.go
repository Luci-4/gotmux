package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"syscall"
	"net"
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
var (
	outConn net.Conn
	inConn net.Conn
)
var (
	inListener net.Listener
	outListener net.Listener
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

func attachToTerminal() {
	log.Println("PTY session started. Awaiting pipe communication.")
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := currentPty.Read(buf)
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
		switch cmd{
		case "exit":
			log.Println("Received 'exit' command. Shutting down.")
			disposeOfCurrentTerminal()
			return
		case "detach":
			log.Println("Received 'detach' command. Detaching.")
			return 
		}

		
		_, err := currentPty.Write([]byte(cmd + "\r"))
		if err != nil {
			log.Printf("Failed to write to PTY: %v", err)
			break
		}
	}
}

func disposeOfCurrentTerminal() {
	currentPty.Close()
	syscall.CloseHandle(currentHandle)
}

func startNewTerminal() {
	pty, err := createConPty(80, 30)
	if err != nil {
		log.Fatalf("Failed to create PTY: %v", err)
	}
	currentPty = pty

	attr := setupChildProcess(currentPty)
	_, handle, err := spawnProcess(currentPty, `C:\Windows\System32\cmd.exe`, attr)
	if err != nil {
		log.Fatalf("Failed to start shell: %v", err)
	}
	currentHandle = syscall.Handle(handle)
	attachToTerminal()
}

func startNamedPipeCommunication() error {
	var err error
	inListener, err = winio.ListenPipe(inputPipeName, nil)
	if err != nil {
		return err
	}

	outListener, err = winio.ListenPipe(outputPipeName, nil)
	if err != nil {
		return err
	}


	log.Println("Waiting for client to connect to output pipe...")
	outConn, err = outListener.Accept()
	if err != nil {
		return err
	}

	log.Println("Client connected to output pipe")

	log.Println("Waiting for client to connect to input pipe...")
	inConn, err = inListener.Accept()
	if err != nil {
		return err
	}

	log.Println("Client connected to input pipe")
	return nil

}

func startServerCommandLoop() {
	scanner := bufio.NewScanner(inConn)
	for scanner.Scan() {
		cmd := scanner.Text()
		switch cmd{
			case "runCMD":
				startNewTerminal()
			case "reattach":
				attachToTerminal()
			case "exit":
				log.Println("Received 'exit' command. Shutting down.")
				return 
			
		}
	}
}

func main() {
	fmt.Println("Starting gotmux server...")

	err := startNamedPipeCommunication()

	if err != nil{
		log.Fatalf("Failed to start named pipe communication: %v", err)
	}

	defer inListener.Close()
	defer outListener.Close()
	defer outConn.Close()
	defer inConn.Close()
	startServerCommandLoop()
	// startNewTerminal()
	log.Println("Session ended.")
}
