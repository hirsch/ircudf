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
	Server    string      //Server address
	sendqueue chan string //Message queue
	conn      net.Conn    //Server connection

	Nickname string //User Nickname
	username string //User username
	realname string //User realname
}

var ( // Events can be changed to custom functions
	EventOnJoin = func(*Server, string, string){}     // server, channel, user
	EventOnPart = func(*Server, string, string, string){} // server, channel, user, message
	EventOnQuit = func(*Server, string, string, string){} // server, channel, user, message
	EventOnPrivmsg = func(*Server, string, string, string){} // server, channel, user, message
	EventOnNotice = func(*Server, string, string, string){} // server, channel, user, message
	EventOnReply = func(*Server, string, string, string){} // server, number, name, reply
)

// Create sets the Server address and user information.
//	// Example: (Nickname!Username@Hostname): Real Name
// 	freenode := ircudf.Create("irc.freenode.org:6667", "Nickname", "Username, "Real Name")
func Create(addr, Nickname, username, realname string) *Server {
	ref := &Server{Server: addr, sendqueue: make(chan string),
		Nickname: Nickname, username: username, realname: realname}
	
	debug("Create:", ref.Server, addr, "\n")
	return ref
}

// Connect establishes a connection to the Server.
// If no parameter is given the default timeout (5 sec) will be used.
func (sock *Server) Connect(timeout ...int) {
	wait := 5 * time.Second
	if len(timeout) >= 1 {
		wait = time.Duration(timeout[0]) * time.Second
	}

	conn, err := net.DialTimeout("tcp", sock.Server, wait)
	sock.conn = conn
	
	errcheck(err) //ADD: RECONNECT
	debug("Connect:", sock.Server, "\n")
}

// Receive receives new messages from the Server and forwards them to parse().
func (sock *Server) Receive() {

	go func() {
		debug("Receive:", sock.Server, "\n")
		reader := bufio.NewReader(sock.conn)
		sock.sendroutine() //Non-Blocking

		for {
			line, err := reader.ReadString('\n')
			errcheck(err)
			debug("->", line)
			sock.parse(strings.Trim(line, "\r\n")) //Remove \r\n for easier parsing
		}

	}()

	time.Sleep(time.Second)
	sock.Nick(sock.Nickname)
	sock.user(sock.username, "0", "0", sock.realname)
}

// parse parses incoming messages from the Server and triggers predefined events.
func (sock *Server) parse(line string) {
	split := strings.SplitN(line, " ", 4)
	split = append(split, make([]string, 4-len(split), 4-len(split))...)

	switch true {
	case split[0] == "PING":
		sock.pong(split[1]) //Ping e.g.: PING :B97B6379
	case split[1] == "PRIVMSG":
		nick := getNick(split[0])
		channel := split[2]
		if channel == sock.Nickname {
			channel = nick
		}
		EventOnPrivmsg(sock, channel, nick, split[3][1:])
	case split[1] == "376":
		sock.Join("#irccs")
	}
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
		}
	}()
}

func getNick(hostmask string) string {
	return strings.Split(hostmask, "!")[0][1:]
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
