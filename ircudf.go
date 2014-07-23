/* Package ircudf implements the Internet Realy Chat (IRC) protocol 
as defined in rfc1459 and provides basic IRC functions
Website: http://tools.ietf.org/html/rfc1459.html*/
package ircudf

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"time"
	"strings"
)

type Server struct {
	server    string
	sendqueue chan string
	conn      net.Conn
	/* USER */
	nickname  string
	username string
	realname string
	/* CHANNEL */
}


/* Create sets the server address and port. Example:
freenode := ircudf.Connect("irc.freenode.org:6667") */
func Create(addr string, nickname string) Server {
	ref := Server{}
	ref.server = addr
	ref.sendqueue = make(chan string)
	ref.nickname = nickname
	debug("Create:", ref.server, addr, "\n")
	return ref
}

/* Connect establishes a connection to the server.
If no parameter is given the default timeout (5 sec) will be used. */
func (sock *Server) Connect(timeout ...int) {
	wait := 5 * time.Second
	if len(timeout) >= 1 {
		wait = time.Duration(timeout[0]) * time.Second
	}

	conn, err := net.DialTimeout("tcp", sock.server, wait)
	sock.conn = conn
	errcheck(err)
	debug("Connect:", sock.server, "\n")
}

/* Receive receives new messages from the server and forwards them to parse(). */
func (sock *Server) Receive() {

	go func() {

		debug("Receive:", sock.server, "\n")
		reader := bufio.NewReader(sock.conn)
		go sock.sendroutine()

		for {
			line, err := reader.ReadString('\n')
			errcheck(err)
			debug("<-", line)
			sock.parse(strings.Trim(line, "\r\n"))	//Remove \r\n for easier parsing
		}

	}()

	time.Sleep(time.Second)
	sock.nick(sock.nickname)
	sock.user(sock.realname, "0", "0", sock.realname)
}


/* parse parses incoming messages from the server and triggers predefined events.*/
func (sock *Server) parse(line string) {
	split := strings.SplitN(line, " ", 4) 
	split = append(split, make([]string, 4-len(split), 4-len(split))...)

	switch true {
	case split[0] == "PING":
		sock.pong(split[1]) //Ping e.g.: PING :B97B6379
	case split[1] == "PRIVMSG" && split[3][1:] == "!ping":
		sock.privmsg(split[2], "!pong")
	case split[1] == "376":
		sock.join("#irccs") 
	}
}

/* nick sets or changes the nickname. The IRC server might reply an errorcode,
if the requsted nickname is not valid or in use. */
func (sock *Server) nick(nickname string) {
	sock.nickname = nickname
	sock.send("NICK " + nickname)
}

/* user specifies the userdata at the beginning of a new connection. 
Servername and hostname are likely to be ignored by the IRC server.
Scheme: (Nickname!Username@Hostname): Real Name*/
func (sock *Server) user(username, hostname, servername, realname string) {
	sock.send("USER " + username + " " + hostname + " " + servername + " :" + realname)
}

/* join joins the specified channel(s). Multiple channels need to be
seperated by a colon. */
func (sock *Server) join(channel string) {
	sock.send("JOIN " + channel )
}

/* pong answers a ping request with a previously received reply string. */
func (sock *Server) pong(reply string) {
	sock.send("PONG " + reply)
}

/* privmsg sends a private message to a user or a channel.
The parameter user can also contain multiple receivers seperated by a colon. */
func (sock *Server) privmsg(user string, message string, ) {
	sock.send("PRIVMSG " + user + " :" + message)
}

 
/* send adds the message to the sendqueue (similar to a FIFO stack) */
func (sock *Server) send(message string) {
	go func() {
		sock.sendqueue <- message + "\n"
	}()
}

/* sendroutine processes the sendqueue and sends the messages to the server.
sendrouting must be executed as goroutine!*/ 
func (sock *Server) sendroutine() {
	for {
		smsg := <-sock.sendqueue
		fmt.Fprint(sock.conn, smsg)
		debug("->", smsg)
	}
}

/* DEBUG FUNCTIONS */

func errcheck(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func debug(s ...interface{}) {
	fmt.Print(s...) //log.Print adds timestamp
}
