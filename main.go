package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"syscall"
	"github.com/charmbracelet/x/conpty"
)

var currentPty *conpty.ConPty
var currentHandle syscall.Handle
var detached bool = false

func createConPty(width, height int) (*conpty.ConPty, error) {
	pty, err := conpty.New(width, height, 0)
	if err != nil {
		log.Fatalf("Failed to create ConPty: %v", err)
	}
	return pty, nil
}

func setupChildProcess(pty *conpty.ConPty) *syscall.ProcAttr {
	attr := &syscall.ProcAttr{
		Files: []uintptr{
			pty.InPipeFd(),
			pty.OutPipeFd(),
			pty.OutPipeFd(),
		},
	}
	return attr
}

func spawnProcess(pty *conpty.ConPty, cmd string, attr *syscall.ProcAttr) (int, uintptr, error) {
	pid, handle, err := pty.Spawn(cmd, []string{}, attr)
	if err != nil {
		log.Fatalf("Failed to spawn process: %v", err)
	}
	return pid, handle, nil
}

func writeCommand(pty *conpty.ConPty, cmd string) error {
	_, err := pty.Write([]byte(cmd))
	if err != nil {
		log.Fatalf("Failed to write to ConPty: %v", err)
	}
	return nil
}



func runCurrentSession(isNew bool) error {
	pty := currentPty
	handle := currentHandle
    defer syscall.CloseHandle(syscall.Handle(handle))

	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := pty.Read(buf)
			if err != nil {
				if err != io.EOF {
					log.Printf("PTY read error: %v", err)
				}
				return
			}
			if n > 0 {
				os.Stdout.Write(buf[:n])
			}
		}
	}()

	scanner := bufio.NewReader(os.Stdin)

	if !isNew {
		_, err := pty.Write([]byte("\r"))
		if err != nil {
			return fmt.Errorf("writing to PTY failed: %w", err)
		}
	}
	for {
		input, err := scanner.ReadString('\n')
		if err != nil {
			return fmt.Errorf("reading stdin failed: %w", err)
		}

		input_trimmed := strings.TrimSpace(input) 
		switch input_trimmed {
		case "exit":
			fmt.Println("Exiting session...")
			return nil

		case "gotmux":
			fmt.Println("Detaching from session...")
            detached = true
			currentPty = pty
			currentHandle = syscall.Handle(handle)
			return nil
		} 

		_, err = pty.Write([]byte(strings.TrimSpace(input) + "\r"))
		if err != nil {
			return fmt.Errorf("writing to PTY failed: %w", err)
		}
	}

	return nil

}

func runNewCMD() error {
    pty, err := createConPty(80, 30)
    if err != nil {
        return err
    }
	currentPty = pty

    attr := setupChildProcess(pty)

    // _, handle, err := spawnProcess(pty, "C:\\Windows\\System32\\WindowsPowerShell\\v1.0\\powershell.exe", attr)
    _, handle, err := spawnProcess(pty, "C:\\Windows\\System32\\cmd.exe", attr)

    if err != nil {
        return err
    }
	currentHandle = syscall.Handle(handle)
	defer func() {
		if !detached {
			pty.Close()
			syscall.CloseHandle(currentHandle)
		}
	}()
	runCurrentSession(true)
	return nil
}

func main() {
    fmt.Println("Welcome to Gotmux")

    for {
        fmt.Print(">>> ") 
        scanner := bufio.NewScanner(os.Stdin)
        if !scanner.Scan() {
            break
        }

        input := strings.TrimSpace(scanner.Text())

        if input == "exit" {
            fmt.Println("Goodbye!")
            break
        }

        switch input {
        case "runCMD":
			fmt.Println("Starting session. Type 'exit' to quit.")
			err := runNewCMD()
			if err != nil {
				log.Printf("Error running session: %v", err)
			}
			fmt.Println("Session ended.")
		case "reattach":
			if currentPty == nil || currentHandle == 0 {
				fmt.Println("No detached session to attach to.")
				break
			}

			fmt.Println("Reattaching to detached session. Type 'gotmux' to detach again, or 'exit' to terminate.")
			detached = false
			err := runCurrentSession(false)
			if err != nil {
				log.Printf("Error reattaching session: %v", err)
			}
        default:
            fmt.Println("Unknown command")
        }
    }
}
