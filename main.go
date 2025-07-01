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

// var currentPty *conpty.ConPty
// var currentHandle syscall.Handle
// var detached bool = false
type Session struct {
	Pty *conpty.ConPty
	Handle syscall.Handle
	Detached bool

}

var currentSession Session

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

	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := currentSession.Pty.Read(buf)
			if err != nil {
				if err != io.EOF {
					fmt.Errorf("PTY read error: %v", err)
					//TODO: log errors somehow
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
		_, err := currentSession.Pty.Write([]byte("\r"))
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
   			syscall.CloseHandle(syscall.Handle(currentSession.Handle))
			currentSession.Pty.Close()
			currentSession.Detached = false
			currentSession.Pty = nil
			currentSession.Handle = 0
			return nil

		case "gotmux":
			fmt.Println("Detaching from session...")
			currentSession.Detached = true
			return nil
		} 

		_, err = currentSession.Pty.Write([]byte(strings.TrimSpace(input) + "\r"))
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
	currentSession.Pty = pty

    attr := setupChildProcess(currentSession.Pty)

    // _, handle, err := spawnProcess(pty, "C:\\Windows\\System32\\WindowsPowerShell\\v1.0\\powershell.exe", attr)
    _, handle, err := spawnProcess(pty, "C:\\Windows\\System32\\cmd.exe", attr)

    if err != nil {
        return err
    }
	currentSession.Handle = syscall.Handle(handle)
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
			if currentSession.Pty == nil || currentSession.Handle == 0 {
				fmt.Println("No detached session to attach to.")
				break
			}

			fmt.Println("Reattaching to detached session. Type 'gotmux' to detach again, or 'exit' to terminate.")
			currentSession.Detached = false
			err := runCurrentSession(false)
			if err != nil {
				log.Printf("Error reattaching session: %v", err)
			}
        default:
            fmt.Println("Unknown command")
        }
    }
}
