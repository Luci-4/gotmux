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
	controlPipeName = `\\.\pipe\gotmux_control`
)

var (
	currentPty    *conpty.ConPty
	currentHandle syscall.Handle
)
var (
	outConn net.Conn
	inConn net.Conn
	controlConn net.Conn
)


var (
	inListener net.Listener
	outListener net.Listener
	controlListener net.Listener
)

var (
    attachStopChan chan struct{}
)


func stopAttach() {
    if attachStopChan != nil {
        close(attachStopChan)
        attachStopChan = nil
    }
}

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
	attachStopChan = make(chan struct{})
    log.Println("PTY session started. Forwarding data.")

    go func() {
        buf := make([]byte, 4096)
        for {
			select {
			case <-attachStopChan:
				return

			default:
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
        }
    }()

    buf := make([]byte, 4096)
    for {
        n, err := inConn.Read(buf)
        if err != nil {
            if err != io.EOF {
                log.Printf("Input pipe read error: %v", err)
            }
            break
        }
        if n > 0 {
            _, err := currentPty.Write(buf[:n])
            if err != nil {
                log.Printf("Failed to write to PTY: %v", err)
                break
            }
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

    err = startControlPipe()
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

func startControlPipe() error {
    var err error
    controlListener, err = winio.ListenPipe(controlPipeName, nil)
    if err != nil {
        return err
    }

    log.Println("Waiting for client to connect to control pipe...")
    controlConn, err = controlListener.Accept()
    if err != nil {
        return err
    }

    log.Println("Client connected to control pipe")
    return nil
}

func controlCommandLoop() {
    scanner := bufio.NewScanner(controlConn)
    for scanner.Scan() {
        cmd := scanner.Text()
		log.Println(cmd)
        switch cmd {
        case "exit":
            log.Println("Received exit command")
            disposeOfCurrentTerminal()
            return
        case "detach":
            log.Println("Received detach command")
			stopAttach()
			clearSequence := "\x1b[2J\x1b[H"
			_, err := outConn.Write([]byte(clearSequence))
			if err != nil {
				log.Printf("Failed to send clear screen sequence: %v", err)
			}
		case "reattach":
            log.Println("Received reattach command")
			go attachToTerminal()
        case "runCMD":
            log.Println("Received runCMD command")
            go startNewTerminal()
        default:
            log.Printf("Unknown control command: %s", cmd)
        }
    }
}

func main() {
    fmt.Println("Starting gotmux server...")

    err := startNamedPipeCommunication()
    if err != nil {
        log.Fatalf("Failed to start named pipe communication: %v", err)
    }

    defer inListener.Close()
    defer outListener.Close()
    defer controlListener.Close()

    defer outConn.Close()
    defer inConn.Close()
    defer controlConn.Close()

    go controlCommandLoop()

    select {} 
}
