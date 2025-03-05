package elevator

import (
	"elevator_system/config"
	"elevator_system/elevio"
	"fmt"
	"time"
)

// **Handles button press events**
func ProcessButtonPress(event elevio.ButtonEvent) {
	fmt.Printf("Button pressed: %+v\n", event)
	elevator.Queue[event.Floor][event.Button] = true
	elevio.SetButtonLamp(event.Button, event.Floor, true)
	HandleStateTransition()
}

// **Handles floor sensor events**
func ProcessFloorArrival(floor int) {
	fmt.Printf("Floor sensor triggered: %+v\n", floor)
	elevator.Floor = floor
	elevio.SetFloorIndicator(floor)

	// If an order exists at this floor, open doors
	if elevator.Queue[floor] != [config.NumButtons]bool{false} {
		fmt.Println("Transitioning from Moving to DoorOpen...")
		elevator.State = config.DoorOpen
		elevio.SetMotorDirection(elevio.MD_Stop)
		elevio.SetDoorOpenLamp(true)

		// **Turn off button lights after servicing**
		elevator.Queue[floor] = [config.NumButtons]bool{false}
		for btn := 0; btn < config.NumButtons; btn++ {
			elevio.SetButtonLamp(elevio.ButtonType(btn), floor, false)
		}
	}
	HandleStateTransition()
}

// **Handles obstruction events**
func ProcessObstruction(obstructed bool) {
	fmt.Printf("Obstruction detected: %+v\n", obstructed)
	elevator.Obstructed = obstructed

	if obstructed {
		elevio.SetMotorDirection(elevio.MD_Stop)
		elevio.SetDoorOpenLamp(true)
		elevator.State = config.DoorOpen
	} else {
		fmt.Println("Obstruction cleared, transitioning to Idle...")
		go func() {
			time.Sleep(config.DoorOpenTime * time.Second)
			if !elevator.Obstructed {
				elevator.State = config.Idle
				HandleStateTransition()
			}
		}()
	}
}

