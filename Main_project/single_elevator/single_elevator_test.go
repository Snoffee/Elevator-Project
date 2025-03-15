package single_elevator

import (
	"Main_project/config"
	"Main_project/elevio"
	"testing"
	"time"
	"Main_project/network"
	"os"
)

// **Test ProcessFloorArrival Using the Simulator**
func TestProcessFloorArrival(t *testing.T) {
	port := os.Getenv("ELEVATOR_PORT")
	if port == "" {
    	port = "15657" // Default
	}
	elevio.Init("localhost:" + port, config.NumFloors)

	InitElevator()

	elevator.Floor = 2
	elevator.Direction = elevio.MD_Up
	elevator.State = config.Moving

	// ✅ Test: Elevator arrives at a floor with both UP and DOWN calls
	elevator.Queue[2][elevio.BT_HallUp] = true
	elevator.Queue[2][elevio.BT_HallDown] = true

	orderStatusChan := make(chan network.OrderStatusMessage)
	ProcessFloorArrival(2, orderStatusChan)

	// 🚀 Step 1: Only UP call should be cleared first
	time.Sleep(1 * time.Second)
	if elevator.Queue[2][elevio.BT_HallUp] {
		t.Errorf("Test Failed: UP call should be cleared immediately")
	}
	if !elevator.Queue[2][elevio.BT_HallDown] {
		t.Errorf("Test Failed: DOWN call should not be cleared immediately")
	}

	// 🚀 Step 2: Wait 3 seconds and check if DOWN call is cleared
	time.Sleep(3 * time.Second)
	if elevator.Queue[2][elevio.BT_HallDown] {
		t.Errorf("Test Failed: DOWN call should be cleared after delay")
	}

	// ✅ Test: Door remains open if obstructed
	elevator.Floor = 1
	elevator.Queue[1][elevio.BT_HallUp] = true
	elevator.Obstructed = true

	ProcessFloorArrival(1, orderStatusChan)
	time.Sleep(3 * time.Second)

	if elevator.State != config.DoorOpen {
		t.Errorf("Test Failed: Door should remain open while obstructed")
	}

	// ✅ Test: Door closes after obstruction is cleared
	elevator.Obstructed = false
	time.Sleep(3 * time.Second)

	if elevator.State == config.DoorOpen {
		t.Errorf("Test Failed: Door should close after obstruction is cleared")
	}
}


