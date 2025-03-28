# Description: This script starts the simulator, elevator, and supervisor for a given elevator ID and port number.
# Usage: ./start_system.sh <ELEVATOR_ID> <ELEVATOR_PORT>
# Example: ./start_system.sh elevator_2 15658
# Note: If no parameters are provided, the script will default to elevator_1 and port 15657.
# chmod +x start_system.sh to make the file executable

#!/bin/bash

# Parameters
ELEVATOR_ID=${1:-"elevator_1"}   # Default to "elevator_1" if not provided
ELEVATOR_PORT=${2:-"15657"}      # Default to "15657" if not provided

echo "Starting Simulator for $ELEVATOR_ID on port $ELEVATOR_PORT..."
gnome-terminal -- bash -c "export SERVER_PORT=$ELEVATOR_PORT; elevatorserver; exec bash" #elevatorserver or filepath to sim ./SimElevatorServer

sleep 2

echo "Building Elevator binary for $ELEVATOR_ID..."
go build -o ../"elevator_$ELEVATOR_ID" main.go
if [ $? -ne 0 ]; then
    echo "Failed to build elevator binary for $ELEVATOR_ID. Exiting..."
    exit 1
fi
echo "Starting Elevator $ELEVATOR_ID on port $ELEVATOR_PORT..."
gnome-terminal -- bash -c "export ELEVATOR_ID=$ELEVATOR_ID; export ELEVATOR_PORT=$ELEVATOR_PORT; ../elevator_$ELEVATOR_ID; exec bash"

sleep 5

echo "Starting Supervisor for $ELEVATOR_ID on port $ELEVATOR_PORT..."
gnome-terminal -- bash -c "export ELEVATOR_ID=$ELEVATOR_ID; export ELEVATOR_PORT=$ELEVATOR_PORT; cd ./supervisor; go run supervisor.go; exec bash"

echo "System started! Simulator, Elevator, and Supervisor are running."