// Package ircudf implements parts of the Internet Realy Chat (IRC) protocol
// as defined in rfc1459 (http://tools.ietf.org/html/rfc1459.html)
// and provides basic IRC functions
package ircudf

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"time"
)

type Server struct { 		// IRC Server
	Server    string        // Server address
	sendqueue chan string   // Message queue
	conn      net.Conn      // Server connection
	throttle  time.Duration // Throttle for message queue

	Nickname string // User Nickname
	username string // User username
	realname string // User realname
}

var Debug = false	// Debug enables logging to stdout

var ( // Events can be changed to custom functions
	eventOnJoin    = func(*Server, string, string) {}         // server, channel, user
	eventOnPart    = func(*Server, string, string, string) {} // server, channel, user, message
	eventOnQuit    = func(*Server, string, string) {}         // server, user, message
	eventOnPrivmsg = func(*Server, string, string, string) {} // server, channel, user, message
	eventOnNotice  = func(*Server, string, string, string) {} // server, channel, user, message
	eventOnReply   = func(*Server, string, string, string) {} // server, number, name, reply
)

// Create sets the Server address and user information.
//	// Example: (Nickname!Username@Hostname): Real Name
// 	freenode := ircudf.Create("irc.freenode.org:6667", "Nickname", "Username, "Real Name")
func Create(addr, Nickname, username, realname string) *Server {
	ref := &Server{Server: addr, sendqueue: make(chan string),
		Nickname: Nickname, username: username, realname: realname,
		throttle: 0}

	debug("Create:", ref.Server, addr, "\n")
	return ref
}

// Connect establishes a connection to the Server.
// If no parameter is given the default timeout (5 sec) will be used.
func (sock *Server) Connect(timeout ...int) error {
	wait := 5 * time.Second
	if len(timeout) >= 1 {
		wait = time.Duration(timeout[0]) * time.Second
	}

	conn, err := net.DialTimeout("tcp", sock.Server, wait)
	sock.conn = conn
	
	if err != nil {
		return err
	}
	
	debug("Connect:", sock.Server, "\n")
	return sock.receive()
}

// Receive receives new messages from the Server and forwards them to parse().
func (sock *Server) receive() error {
	go func() {
		time.Sleep(time.Second)
		sock.Nick(sock.Nickname)
		sock.user(sock.username, "0", "0", sock.realname)
	}()

	debug("Receive:", sock.Server, "\n")
	reader := bufio.NewReader(sock.conn)
	sock.sendroutine() //Non-Blocking

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return err
		}
		debug("->", line)
		sock.parse(strings.Trim(line, "\r\n")) //Remove \r\n for easier parsing
	}
}

// parse parses incoming messages from the Server and triggers predefined events.
func (sock *Server) parse(line string) {
	split := strings.SplitN(line, " ", 4)
	split = append(split, make([]string, 4-len(split), 4-len(split))...)

	switch true {
	case split[0] == "PING":
		sock.pong(split[1]) //Ping e.g.: PING :B97B6379
	case split[1] == "JOIN":
		eventOnJoin(sock, split[2][1:], getNick(split[0]))
	case split[1] == "PART":
		eventOnPart(sock, split[2], getNick(split[0]), split[3][1:])
	case split[1] == "QUIT":
		if split[3] != "" {
			split[2] += " " + split[3]
		}
		eventOnQuit(sock, getNick(split[0]), split[2][1:])
	case split[1] == "PRIVMSG":
		nick := getNick(split[0])
		channel := split[2]
		if channel == sock.Nickname {
			channel = nick
		}
		eventOnPrivmsg(sock, channel, nick, split[3][1:])
	case split[1] == "NOTICE":
		nick := getNick(split[0])
		channel := split[2]
		if channel == sock.Nickname {
			channel = nick
		}
		eventOnNotice(sock, channel, nick, split[3][1:])
	case isNum(split[1]):
		eventOnReply(sock, split[1], split[2], split[3])
	}
}

// Throttle sets the delay between sending messages in milliseconds.
func (sock *Server) Throttle(delay int) {
	sock.throttle = time.Duration(delay) * time.Millisecond
}

// Nick sets or changes the Nickname. The IRC Server might reply an errorcode,
// if the requsted Nickname is not valid or in use.
func (sock *Server) Nick(Nickname string) {
	sock.Nickname = Nickname
	sock.Send("NICK " + Nickname)
}

// user specifies the userdata at the beginning of a new connection.
// Servername and hostname are likely to be ignored by the IRC Server.
// Scheme: (Nickname!Username@Hostname): Real Name
func (sock *Server) user(username, hostname, Servername, realname string) {
	sock.Send("USER " + username + " " + hostname + " " + Servername + " :" + realname)
}

// Join joins the specified channel(s). Multiple channels need to be
// seperated by a colon.
func (sock *Server) Join(channel string) {
	sock.Send("JOIN " + channel)
}

// pong answers a ping request with a previously received reply string.
func (sock *Server) pong(reply string) {
	sock.Send("PONG " + reply)
}

// Privmsg sends a private message to a user or a channel.
// The parameter user can also contain multiple receivers seperated by a colon.
func (sock *Server) Privmsg(user string, message string) {
	sock.Send("PRIVMSG " + user + " :" + message)
}

// Notice sends a notice to a user or a channel.
// The parameter user can also contain multiple receivers seperated by a colon.
func (sock *Server) Notice(user string, message string) {
	sock.Send("NOTICE " + user + " :" + message)
}

// Send adds the message to the RAW Message queue
func (sock *Server) Send(message string) {
	go func() {
		sock.sendqueue <- message + "\n"
	}()
}

// sendroutine sends the messages of the queue to the Server.
func (sock *Server) sendroutine() {
	go func() {
		for {
			smsg := <-sock.sendqueue
			io.WriteString(sock.conn, smsg)
			debug("<-", smsg)
			time.Sleep(sock.throttle)
		}
	}()
}

// getNick extracts the nickame out of an hostmask. This function is
// being used for parsing incoming messages.
func getNick(hostmask string) string {
	return strings.Split(hostmask, "!")[0][1:]
}

// isNum checks wether a string could be represented as integer.
// This function is being used for parsing incoming messages.
func isNum(str string) bool {
	_, err := strconv.Atoi(str)
	return err == nil
}

//HANDLE FUNCTIONS

// HandleJoin registers the handle function for join.
// Handle function parameters: server, channel, user
func HandleJoin(h func(*Server, string, string)) {
	eventOnJoin = h
}

// HandlePart registers the handle function for part.
// Handle function parameters: server, channel, user, message
func HandlePart(h func(*Server, string, string, string)) {
	eventOnPart = h
}

// HandleQuit registers the handle function for quit.
// Handle function parameters: server, user, message
func HandleQuit(h func(*Server, string, string)) {
	eventOnQuit = h
}

// HandlePrivmsg registers the handle function for privmsg.
// Handle function parameters: server, channel, user, message
func HandlePrivmsg(h func(*Server, string, string, string)) {
	eventOnPrivmsg = h
}

// HandleNotice registers the handle function for notice.
// Handle function parameters: server, channel, user, message
func HandleNotice(h func(*Server, string, string, string)) {
	eventOnNotice = h
}

// HandleReply registers the handle function for reply.
// Handle function parameters: server, number, name, reply
func HandleReply(h func(*Server, string, string, string)) {
	eventOnReply = h
}

// DEBUG FUNCTIONS

func debug(s ...interface{}) {
	if Debug { 
		fmt.Print(s...)
	}
}
