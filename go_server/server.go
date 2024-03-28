package main

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"regexp"
	"strings"
	"sync"
	"unicode"
)

// BroadcastType represents the type of message to be broadcast to all other chat users
type BroadcastType int

const (
	UserJoinsServer BroadcastType = iota
	UserChangesNickname
	UserLeavesServer
)

// ChatServer represents a server capable of handling chat messages between users.
type ChatServer struct {
	users map[net.Conn]string // users maps network connections to user nicknames
	mutex sync.Mutex          // mutex protects access to the users map
}

const (
	HOST = "localhost"
	PORT = "4000"
	TYPE = "tcp"

	LIST = "/LIST"
	NICK = "/NICK"
	MSG  = "/MSG"
)

// RegExp defined as global variable, so it's compiled once when program starts
var validNicknamePattern = regexp.MustCompile("^[a-zA-Z0-9_]+$")

// start initiates the chat server, listening for incoming TCP connections on the predefined host and port.
// New connections are handled concurrently in separate goroutines.
func (chatServer *ChatServer) start() {

	listen, err := net.Listen(TYPE, HOST+":"+PORT)
	if err != nil {
		log.Fatalf("Failed to start server: %v\n", err)
	}

	defer listen.Close()

	log.Printf("Server started on %s:%s\n", HOST, PORT)

	for {
		conn, err := listen.Accept()
		if err != nil {
			log.Printf("There was a problem connecting: %v\n", err)
			continue
		}
		go chatServer.handleClientConnection(conn)
	}
}

// handleClientConnection manages a single client connection, reading commands and responding appropriately.
// It ensures the connection is closed when the function returns and broadcasts a disconnect message if applicable.
func (server *ChatServer) handleClientConnection(conn net.Conn) {

	log.Printf("Client %s connected to server\n", conn.RemoteAddr().String())

	defer conn.Close()

	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		sanitizedUserCommand := strings.Trim(scanner.Text(), " ")
		server.handleUserCommands(sanitizedUserCommand, conn)
	}

	// Check if client has left server; if so, delete them from client list
	if err := scanner.Err(); err != nil {
		log.Printf("Error reading from %s: %v", conn.RemoteAddr(), err)

	} else {
		log.Printf("Client %s disconnected\n", conn.RemoteAddr())
		server.broadcastMsg(UserLeavesServer, conn, server.users[conn])
	}

	server.mutex.Lock()
	delete(server.users, conn)
	server.mutex.Unlock()
}

// handleUserCommands interprets and processes commands received from a user.
// Supported commands are /NICK for setting a nickname, /LIST for listing users, and /MSG for messaging.
func (server *ChatServer) handleUserCommands(userCommand string, conn net.Conn) {

	args := strings.SplitN(userCommand, " ", 3)

	switch {

		case len(args) >= 1 && args[0] == LIST:
			server.handleListCommand(conn)

		case len(args) >= 2 && args[0] == NICK:
			desiredNickname := args[1]
			server.handleNicknameCommand(conn, desiredNickname)

		case len(args) >= 3 && args[0] == MSG:
			recipients := args[1]
			message := args[2]
			server.handleMessageCommand(conn, recipients, message)

		default:
			fmt.Fprintln(conn, "Invalid command")
	}
}

// handleListCommand sends a list of currently connected users to the requesting client.
func (server *ChatServer) handleListCommand(conn net.Conn) {

	server.mutex.Lock()
	defer server.mutex.Unlock()

	fmt.Fprint(conn, "Current users: ")

	for _, nickname := range server.users {
		fmt.Fprint(conn, nickname, " ")
	}
	fmt.Fprintln(conn)
}

// handleNicknameCommand processes a request from a client to set or change their nickname,
// ensuring the nickname is valid and not already in use.
func (server *ChatServer) handleNicknameCommand(conn net.Conn, desiredNickname string) {

	validNickname, msg := validateNickname(desiredNickname)
	if !validNickname {
		fmt.Fprintln(conn, msg)
		return
	}

	server.mutex.Lock()
	defer server.mutex.Unlock()

	for userConn, userNickname := range server.users {
		if userNickname == desiredNickname {
			if userConn == conn {
				fmt.Fprintf(conn, "You're already registered as %s\n", desiredNickname)
			} else {
				fmt.Fprintf(conn, "%s already registered\n", desiredNickname)
			}
			return
		}
	}

	if currentNickname, exists := server.users[conn]; exists {
		fmt.Fprintf(conn, "You changed your nickname from %s to %s\n", currentNickname, desiredNickname)
		server.broadcastMsg(UserChangesNickname, conn, currentNickname, desiredNickname)

	} else {
		fmt.Fprintf(conn, "Nickname registered as %s\n", desiredNickname)
		server.broadcastMsg(UserJoinsServer, conn, desiredNickname)
	}

	server.users[conn] = desiredNickname
}

// validateNickname checks if the provided nickname is valid according to predefined rules.
// It must start with a letter, contain only letters, numbers, and underscores, and be 1-10 characters long.
func validateNickname(nickname string) (bool, string) {

	sanitizedNickname := strings.Trim(nickname, " ")

	if len(sanitizedNickname) < 1 || len(sanitizedNickname) > 10 {
		return false, "Nickname must be between 1 and 10 characters"
	}

	firstLetter := rune(sanitizedNickname[0])
	if !unicode.IsLetter(firstLetter) {
		return false, "Nickname must start with a letter"
	}

	if !validNicknamePattern.MatchString(sanitizedNickname) {
		return false, "Nickname can contain only letters, numbers, and underscores"
	}

	return true, ""
}

// handleMessageCommand handles messaging commands, allowing a user to send a message to all users or specified users.
func (server *ChatServer) handleMessageCommand(conn net.Conn, recipients string, message string) {

	parsedRecipients := strings.Split(recipients, ",")
	senderNickname := server.users[conn]

	if senderNickname == "" {
		fmt.Fprintln(conn, "You must register a nickname before you can send a message")
		return
	}

	switch {

		case len(parsedRecipients) == 1 && parsedRecipients[0] == "*":
			server.sendToAllUsers(conn, senderNickname, message)

		default:
			server.sendToSpecificUsers(conn, senderNickname, parsedRecipients, message)
	}
}

func (server *ChatServer) sendToAllUsers(conn net.Conn, senderNickname string, message string) {

	server.mutex.Lock()
	defer server.mutex.Unlock()

	// Sender does not receive their own broadcast message
	for connection := range server.users {
		if connection != conn {
			fmt.Fprintf(connection, "%s said: %s\n", senderNickname, message)
		}
	}
}

func (server *ChatServer) sendToSpecificUsers(conn net.Conn, senderNickname string, recipients []string, message string) {

	server.mutex.Lock()
	defer server.mutex.Unlock()

	for _, receiver := range recipients {
		for receiverConnection, receiverNickname := range server.users {

			// Sender cannot message themselves
			if receiverNickname == receiver && conn != receiverConnection {
				fmt.Fprintf(receiverConnection, "%s said: %s\n", senderNickname, message)
			}
		}
	}
}

func (server *ChatServer) broadcastMsg(broadcastType BroadcastType, excludeConn net.Conn, components ...string) {

	var message string

	switch broadcastType {

		case UserJoinsServer:
			message = fmt.Sprintf("%s joined the chat", components[0])

		case UserLeavesServer:
			message = fmt.Sprintf("%s left the chat", components[0])

		case UserChangesNickname:
			message = fmt.Sprintf("%s changed nickname to %s", components[0], components[1])

		default:
			log.Println("Unknown broadcast type")
			return
	}

	// User doing action doesn't receive message
	for conn := range server.users {
		if conn != excludeConn {
			fmt.Fprintln(conn, message)
		}
	}
}

func main() {

	chatServer := ChatServer{
		users: make(map[net.Conn]string),
	}

	chatServer.start()
}
