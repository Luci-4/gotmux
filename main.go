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


func handleIO(pty *conpty.ConPty, inputChan chan string) {
    buf := make([]byte, 4096)

    go func() {
        for {
            n, err := pty.Read(buf)
            if err != nil {
                if err != io.EOF {
                    log.Printf("Read error: %v", err)
                }
                return
            }
            if n > 0 {
                os.Stdout.Write(buf[:n])
            }
        }
    }()

    for {
        input, ok := <-inputChan
        if !ok {
            log.Println("Input channel closed")
            return
        }
        err := writeCommand(pty, input+"\n")
        if err != nil {
            log.Printf("write error: %v", err)
            return
        }
    }
}

func runNewCMD() error {
    pty, err := createConPty(80, 30)
    if err != nil {
        return err
    }
    defer pty.Close()

    attr := setupChildProcess(pty)

    // _, handle, err := spawnProcess(pty, "C:\\Windows\\System32\\WindowsPowerShell\\v1.0\\powershell.exe", attr)
    _, handle, err := spawnProcess(pty, "C:\\Windows\\System32\\cmd.exe", attr)

    if err != nil {
        return err
    }
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
	for {
		input, err := scanner.ReadString('\n')
		if err != nil {
			return fmt.Errorf("reading stdin failed: %w", err)
		}

		if strings.TrimSpace(input) == "exit" {
			fmt.Println("Exiting PowerShell session...")
			break
		}

		_, err = pty.Write([]byte(strings.TrimSpace(input) + "\r"))
		if err != nil {
			return fmt.Errorf("writing to PTY failed: %w", err)
		}
	}

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
			fmt.Println("Starting PowerShell session. Type 'exit' to quit.")
			err := runNewCMD()
			if err != nil {
				log.Printf("Error running PowerShell: %v", err)
			}
			fmt.Println("PowerShell session ended.")
        default:
            fmt.Println("Unknown command")
        }
    }
}
