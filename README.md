# **Elevator-Project**
TTK4145 Sanntidsprogramering

## **Overview**
This project implements a **multi-elevator system** that can:
- **Process cab and hall calls** efficiently.
- **Assign orders dynamically** using an election-based master system.
- **Detect network failures** and **reassign lost orders** automatically.

The system is built using **Go** and follows a **modular architecture** with clear communication between modules using **Go channels**.

---

## **Project Structure**
| **Module**           | **Description** |
|----------------------|----------------|
| `main`            | Initializes the system, sets up channels, and starts goroutines. |
| `singleElevator` | Handles individual elevator logic (cab calls, movement, state transitions). |
| `orderAssignment` | Assigns hall calls based on elevator states and reassigns lost orders. |
| `masterElection` | Elects a master elevator and ensures consistent master updates. |
| `peerMonitor`    | Monitors connected elevators and detects failures. |
| `network`         | Handles peer communication and broadcasts elevator states. |
| `config`          | Defines shared configurations and constants. |
| `elevio`          | Bridge between code and physical elevator. |
| `communication`   | Handles message sending, elevator status updates and generally manages network functionality. |
| `supervisor`   | Restarts the elevator when it enters a failure state. |


---

## **Architecture**

- **Master-Slave Model:**
The system operates in a master-slave configuration. One elevator is elected as the master, which is responsible for handling hall call assignments, order distribution and reassigning of lost hall calls. In addition, it sends a backup of a resurrected slaves previous cab calls. Other elevators act as slaves, executing assigned hall orders.

- **Master Election:**
If the current master goes offline, the elevator with the lowest ID of the remaining is elected as the new master. A reelection is held every time a new peer enters the network.

- **Communication Protocol:**
All elevators communicate using UDP broadcasting, ensuring that network messages such as peer updates, master elections, and order assignments are efficiently shared.

- **Acknowledgement System:**
All messages are equipped with individual sequence numbers and confirmed by the recipient sending an acknowledgement message with the same sequence number to the transmitter. The transmitter keeps resending messages untill an acknowledgement is received or it times out.

- **Supervisor:**
Each elevator has its own supervisor that keeps tabs on the executable. It detects when the executable is down and automatically restarts it. Used to handle failure states, like loss of motor power and obstruction problems.

---

## Modules Inputs and Outputs
(Network module excluded as it only provides framework for UDP broadcasting)

| Module         | Inputs                                                         | Outputs                                                        |
|----------------|---------------------------------------------------------------|----------------------------------------------------------------|
| `singleElevator`| I/O Events, Network messages (Assignments, Raw Hall Calls, Light Orders, Status Messages). | localStatusUpdate , Sends order status messages, acknowledgments, Operates motors, lamps, and door control. |
| `orderAssignment`| Elevator Statuses, Master Election Results, Lost/Recovered Peers, Hall Call Requests.  | Sends Assignments, Reassigns and restores Lost Orders, Forwards raw hall calls to master. |
| `masterElection`| Elevator Statuses.                        				| New master (`MasterID`). |
| `peerMonitor`   | Network peer updates (New and Lost).                        	  | Sends notification of lost and recovered peers to `orderAssignment`. |
| `config`        | Environment variables.     						 | Global `LocalID`, `MasterID`, and constants (`NumFloors`, `NumButtons`). Initializes elevator |
| `elevio`        | Hardware commands.| Provides button press events, floor sensor events, obstruction events. Writes to hardware interface. |
| `communication` |Elevator Status Updates, Order Status, Acks. 			 | Ensures reliable transmission of messages with acknowledgments and retries. Broadcasts Elevator Statuses periodically and in bursts at critical events |
| `supervisor`    | Fault detection (Timeout events).                             | Restarts elevator when its down. |

---

## **Hall Button Press Lifecycle**
Different handling of hall button press based on the source elevator. Slaves forward hall call to master while master passes it to order assignment. If master is the best elevator for the order it is passed on to its assignedHallCallChan. If not it is sent on the network via the txAssignmentChan.
![485081540_840947238219688_7134016836677410224_n](https://github.com/user-attachments/assets/1c4f5583-07be-462f-a256-ce58df9f434a)


---

## **Message Types**
We made an overview of senders and receivers of messages to keep ping pong packets to a minimum. 

The receiver of a message sends AckMessage to sender. 

| 	               | **Sender** | **Receiver** |
|----------------------|------------|--------------|
| `LightOrderMessage`  | Master👑 | NOT: Master👑 and recipient of assignment |
| `AssignmentMessage` | Master👑 | ALL |
| `RawHallCallMessage` | Slaves | Master👑 |
| `OrderStatusMessage` | Slaves (Master via chan) | Master👑 |

Recently received messages are kept in individual maps based on type, to ensure no duplication of execution. Due to our resending mechanism, the same message can be received multiple times if acknowledgment packets are lost on the network. These maps block duplicates from being processed again and potentially causing unwanted behaviour.

---

## **Setup and Running the Project**
Set up environment variables for each elevator when running multiple elevators on the same machine. Specify the unique elevator ID and port number:

- **Windows PowerShell**
	- Terminal 1: 	
	- $env:ELEVATOR_PORT="15657"
	- $env:ELEVATOR_ID="elevator_1"

  	- Terminal 2
	- $env:ELEVATOR_PORT="15658"
	- $env:ELEVATOR_ID="elevator_2"

- **Linux (or macOS)**
	- Terminal 1:
	- export ELEVATOR_PORT="15657"
	- export ELEVATOR_ID="elevator_1"

	- Terminal 2
	- export ELEVATOR_PORT="15658"
	- export ELEVATOR_ID="elevator_2"

To start the elevator system:
- go run main.go

## **Using the script**
Additionally you can start an elevator with a corresponding simulator and supervisor by running the script. If no parameters are provided, the script will default to elevator_1 and port 15657

- **Windows PowerShell**
	- Set-ExecutionPolicy -ExecutionPolicy RemoteSigned -Scope CurrentUser
	- .\start_system.ps1 -ElevatorID "elevator_1" -ElevatorPort "15657"

- **Linux (or macOS)**
	- chmod +x start_system.sh to make the file executable
	- ./start_system.sh elevator_1 15657
