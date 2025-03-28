package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"runtime"
	"time"
)

var (
    elevatorID   = os.Getenv("ELEVATOR_ID")   
    elevatorPort = os.Getenv("ELEVATOR_PORT") 
)

func main() {
	if elevatorID == "" || elevatorPort == "" {
		log.Fatal("ELEVATOR_ID or ELEVATOR_PORT is not set! Exiting...")
	}

	log.Printf("Supervisor started for Elevator %s on port %s.", elevatorID, elevatorPort)

	go monitorElevator()
	select {} 
}

// Monitor a Single Elevator via Peer Network
func monitorElevator() {
	for {
        // Check if the elevator process is running
        if !isElevatorRunning(elevatorID) {
            log.Printf("Elevator %s is not running! Restarting...", elevatorID)
            go restartElevator(elevatorID)
        }
        time.Sleep(5 * time.Second) // Check every 5 seconds
    }
}

func isElevatorRunning(elevatorID string) bool {
    var cmd *exec.Cmd

    if runtime.GOOS == "windows" {
        psCommand := fmt.Sprintf(`Get-Process | Where-Object { $_.Name -eq "elevator_%s" }`, elevatorID)
        cmd = exec.Command("powershell", "-Command", psCommand)
    } else if runtime.GOOS == "linux" {
        cmd = exec.Command("pgrep", "-f", fmt.Sprintf("elevator_%s", elevatorID))
    }

    output, err := cmd.Output()
    if err != nil {
        log.Printf("Elevator %s is not running: %v", elevatorID, err)
        return false
    }

    log.Printf("Command output: %s", string(output))
    if len(output) == 0 {
        log.Printf("Elevator %s is not running (no output from command).", elevatorID)
        return false
    }

    log.Printf("Elevator %s is running.", elevatorID)
    return true
}

func restartElevator(elevatorID string) {
	log.Printf("Restarting Elevator: %s on port %s...", elevatorID, elevatorPort)
	var cmd *exec.Cmd

    if runtime.GOOS == "windows" {
        psCommand := fmt.Sprintf(`Start-Process powershell -WindowStyle Normal -ArgumentList '-Command', 'cd ..; $env:ELEVATOR_ID=\"%s\"; $env:ELEVATOR_PORT=\"%s\"; ./elevator_%s.exe'`, elevatorID, elevatorPort, elevatorID)
        cmd = exec.Command("powershell", "-Command", psCommand)
    } else if runtime.GOOS == "linux" {
        bashCommand := fmt.Sprintf(`cd ../.. && ELEVATOR_ID="%s" ELEVATOR_PORT="%s" ./elevator_%s`, elevatorID, elevatorPort, elevatorID)
        cmd = exec.Command("gnome-terminal", "--", "bash", "-c", bashCommand)
	}
	cmd.Stdout = os.Stdout
    cmd.Stderr = os.Stderr

	cmd.Start()

	// Add a longer delay after restarting
    time.Sleep(30 * time.Second) 
}