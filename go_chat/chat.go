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

const (
	HOST = "localhost"
	PORT = "4000"
	TYPE = "tcp"
)

const onlyLettersNumbersUnderscore = "^[a-zA-Z0-9]+$"

var (
	clients = make(map[net.Conn]string)
	mutex   = sync.Mutex{}
)


func main() {

	// Attempt to start a server
	listen, err := net.Listen(TYPE, HOST + ":" + PORT)

	/* If there's an error, then the server cannot start.
	Log and print error; call os.exit() to end program. */
	if err != nil {
		log.Fatal(err)
	}

	/* Once server has successfully started, create a map
	that will allow us to store users.
	> net.Conn is the client connection (client ID)
	> string is the client nickname */



	/* Server has started; begin listening for TCP requests.*/
	for {
		conn, err := listen.Accept()

		// If error accepting connection, log it and continue listening
		if err != nil {
			log.Printf("There was a problem connecting: %v\n", err)
			continue
		}

		// If connection, handle it on a specific goroutine.
		go handleConnection(conn)
	}
}

func handleConnection(conn net.Conn) {

	// Ensures the connection is closed when function returns
	defer conn.Close()

	// Creates a new scanner to read input from the connection
	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {

		// Sanitize data by removing whitespace
		sanitizedData := strings.Trim(scanner.Text(), " ")
		handleCommands(sanitizedData, conn)
	}
}

func handleCommands(command string, conn net.Conn) {

	// Max valid length is 3
	splitCommand := strings.SplitN(command, " ", 3)

	if len(splitCommand) == 1 && splitCommand[0] == "/LIST" {
		returnListOfUsers(conn)

	} else if len(splitCommand) == 2 && splitCommand[0] == "/NICK" {
		userNickname := splitCommand[1]
		setUserNickname(conn, userNickname)

	} else if len(splitCommand) == 3 && splitCommand[0] == "/MSG" {
		sendMessage(conn)

	} else {
		// Return error message to client
		fmt.Fprintln(conn, "ERROR!")
	}

}

func returnListOfUsers(conn net.Conn) {
	fmt.Fprintln(conn, "Returning list of users...")
}

func setUserNickname(conn net.Conn, nickname string) {

	// Ensure nickname is valid
	valid, msg := validateNickname(nickname)
	if !valid {
		fmt.Fprintln(conn, msg)
		return
	}

	mutex.Lock()
	defer mutex.Unlock()

	/* Check if user trying to re-register the same nickname.
	If so, notify user; if not, notify user they're changing nickname. */
	if currentNickname, exists := clients[conn]; exists {
		if currentNickname == nickname {
			fmt.Fprintf(conn, "You're already registered as %s!\n", nickname)
			return
		} else {
			fmt.Fprintf(conn, "Changing nickname from %s to %s\n", currentNickname, nickname)
		}
	}

	// Check if nickname already registered
	for _, name := range clients {
		if name == nickname  {
			fmt.Fprintf(conn, "%s already registered\n", nickname)
			return
		}
	}

	clients[conn] = nickname
	fmt.Fprintf(conn, "Nickname registered as %s\n", nickname)
}

func sendMessage(conn net.Conn) {
	fmt.Fprintln(conn, "Sending message...")
}

func validateNickname(nickname string) (bool, string) {

	matched, _ := regexp.MatchString(onlyLettersNumbersUnderscore, nickname)
	if !matched {
		return false, "Nickname must contain only letters, numbers, and underscores"
	}

	if len(nickname) < 1 || len(nickname) > 10 {
		return false, "Nickname must be between 1 and 10 characters"
	}

	if !unicode.IsLetter(rune(nickname[0])) {
		return false, "Nickname must start with a letter"
	}

	return true, "Nickname is valid"
}

// TODO: Remove user when they disconnect.
