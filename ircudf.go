// Package ircudf implements parts of the Internet Realy Chat (IRC) protocol
// as defined in rfc1459 (http://tools.ietf.org/html/rfc1459.html)
// and provides basic IRC functions
package ircudf

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"strings"
	"time"
	"io"
)

type Server struct { //IRC Server
	server    string      //Server address
	sendqueue chan string //Message queue
	conn      net.Conn    //Server connection

	nickname string //User nickname
	username string //User username
	realname string //User realname
}

var ( // Events can be changed to custom functions
	EventOnJoin    func(string, string)         // channel, user
	EventOnPart    func(string, string, string) // channel, user, message
	EventOnPrivmsg func(string, string, string) // channel, user, message
	EventOnNotice  func(string, string, string) // channel, user, message
	EventOnReply   func(string, string, string) // number, name, reply
)

// Create sets the server address and user information. Example:
// 	freenode := ircudf.Connect("irc.freenode.org:6667", "jmiller", "jmiller, "John Miller")
func Create(addr, nickname, username, realname string) Server {
	ref := Server{server: addr, sendqueue: make(chan string),
		nickname: nickname, username: username, realname: realname}
	
	debug("Create:", ref.server, addr, "\n")
	return ref
}

// Connect establishes a connection to the server.
// If no parameter is given the default timeout (5 sec) will be used.
func (sock *Server) Connect(timeout ...int) {
	wait := 5 * time.Second
	if len(timeout) >= 1 {
		wait = time.Duration(timeout[0]) * time.Second
	}

	conn, err := net.DialTimeout("tcp", sock.server, wait)
	sock.conn = conn
	
	errcheck(err) //ADD: RECONNECT
	debug("Connect:", sock.server, "\n")
}

// Receive receives new messages from the server and forwards them to parse().
func (sock *Server) Receive() {

	go func() {
		debug("Receive:", sock.server, "\n")
		sock.sendroutine() //Non-Blocking
		reader := bufio.NewReader(sock.conn)

		for {
			line, err := reader.ReadString('\n')
			errcheck(err)
			debug("<-", line)
			sock.parse(strings.Trim(line, "\r\n")) //Remove \r\n for easier parsing
		}

	}()

	time.Sleep(time.Second)
	sock.Nick(sock.nickname)
	sock.user(sock.username, "0", "0", sock.realname)
}

// parse parses incoming messages from the server and triggers predefined events.
func (sock *Server) parse(line string) {
	split := strings.SplitN(line, " ", 4)
	split = append(split, make([]string, 4-len(split), 4-len(split))...)

	switch true {
	case split[0] == "PING":
		sock.pong(split[1]) //Ping e.g.: PING :B97B6379
	case split[1] == "PRIVMSG" && split[3][1:] == "!ping":
		sock.Privmsg(split[2], "!pong")
	case split[1] == "376":
		sock.Join("#irccs")
	}
}

// Nick sets or changes the nickname. The IRC server might reply an errorcode,
// if the requsted nickname is not valid or in use.
func (sock *Server) Nick(nickname string) {
	sock.nickname = nickname
	sock.Send("NICK " + nickname)
}

// user specifies the userdata at the beginning of a new connection.
// Servername and hostname are likely to be ignored by the IRC server.
// Scheme: (Nickname!Username@Hostname): Real Name
func (sock *Server) user(username, hostname, servername, realname string) {
	sock.Send("USER " + username + " " + hostname + " " + servername + " :" + realname)
}

// Join joins the specified channel(s). Multiple channels need to be
// seperated by a colon. */
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

// Send adds the message to the RAW Message queue
func (sock *Server) Send(message string) {
	go func() {
		sock.sendqueue <- message + "\n"
	}()
}

// sendroutine processes the sendqueue and sends the messages to the server.
// sendrouting must be executed as goroutine!
func (sock *Server) sendroutine() {
	go func() {
		for {
			smsg := <-sock.sendqueue
			//fmt.Fprint(sock.conn, smsg)
			io.WriteString(sock.conn, smsg) 
			debug("->", smsg)
		}
	}()
}

// DEBUG FUNCTIONS

func errcheck(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func debug(s ...interface{}) {
	fmt.Print(s...) //log.Print adds timestamp
}
