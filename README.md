# Reverse Shell
- Simple reverse shell implemented using Go
- This is not a secure channel and should only be used for testing purposes, do not leave it running on your device
- Whenever user input is allows, it uses the deliminator ">" to symbolize it is waiting for input

# Notes
- Open the port on the target firewall to expose the reverse shell
- There is no encryption on the traffic

# To Run
1. Use Go to run the source code with "go run main.go" or build and run the executable
2. The program will listen on localhost port 6060 by default; to connect from another device, you likely need to open the port on the firewall
3. Connect to the device on the specific port, use like a regular shell

# Challenges
- Starting the delimiter at the beginning of the line, even if output comes late: solution is using a carriage return to rewrite the line and replace the delim
- Whenever you are asked for input, there will be a delim
- To use from host computer, you would need to expose the port number through the firewall

# TODO
- Handle EOF, client disconnects themselves
- Sync map for user database

# Errors to Handle
- With two concurrent connections
    2023/05/07 19:20:49 reading loop:       write tcp 127.0.0.1:6060->127.0.0.1:36168: write: broken pipe