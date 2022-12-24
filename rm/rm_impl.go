package main

import (
	"bufio"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/fatih/color"
)

const (
	MAX_WRITE_SIZE = 500
	GFD_PORT       = 8003
	LIMIT          = 5
	C1_PORT        = 8006
	C2_PORT        = 8007
	C3_PORT        = 8008
)

type rm struct {
	acceptClose  chan bool // chan of request accept routine close
	acceptClosed chan bool // chan of accept routine closed
	mainClose    chan bool // chan of request main routine close
	mainClosed   chan bool // chan of main routine closed
	gfd          clientGFD
	servers      []server
	clients      []clientGFD
	IP           string
	config       string
	active       bool
	accept       chan connect // chan of accepted connection
	readChan     chan request
	name         string
	memColor     *color.Color
	sendColor    *color.Color
	primColor    *color.Color
}

type server struct {
	name    string
	deleted bool
	primary bool
}
type clientGFD struct {
	conn net.Conn // server's connection
	name string
}

type connect struct {
	conn net.Conn // lfd's connection
	name string   // lfd name
}

type request struct {
	client  clientGFD // the destionation of the request
	request string    // request's text
}

func NewRM() RM {
	r := rm{
		acceptClose:  make(chan bool),
		acceptClosed: make(chan bool),
		mainClose:    make(chan bool),
		mainClosed:   make(chan bool),
		accept:       make(chan connect),
		readChan:     make(chan request),
	}
	return &r
}
func (r *rm) Close() {
	r.mainClose <- true
	<-r.mainClosed
}

func (r *rm) Start(name string, config string, ip string) error {
	r.IP = ip
	if config == "active" {
		r.active = true
	} else {
		r.active = false
	}
	r.sendColor = color.New(color.FgBlue).Add(color.Bold)
	r.memColor = color.New(color.FgRed).Add(color.Bold)
	r.primColor = color.New(color.FgBlue).Add(color.Bold)

	r.config = config
	r.name = "RM"
	err := r.tcpStart()
	go r.mainRoutine()
	if err != nil {
		return err
	}
	fmt.Println("Started RM with config: " + r.config)

	return nil
}

func (r *rm) tcpStart() error {
	ls, err := net.Listen("tcp", r.IP+":"+strconv.Itoa(GFD_PORT))
	if err != nil {
		return err
	}
	go r.acceptRoutine(ls, "GFD")

	ls, err = net.Listen("tcp", r.IP+":"+strconv.Itoa(C1_PORT))
	if err != nil {
		return err
	}
	go r.acceptRoutine(ls, "C1")

	ls, err = net.Listen("tcp", r.IP+":"+strconv.Itoa(C2_PORT))
	if err != nil {
		return err
	}
	go r.acceptRoutine(ls, "C2")

	ls, err = net.Listen("tcp", r.IP+":"+strconv.Itoa(C3_PORT))
	if err != nil {
		return err
	}
	go r.acceptRoutine(ls, "C3")
	return nil
}

func (r *rm) acceptRoutine(listener net.Listener, name string) {
	for {
		select {
		default:
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			fmt.Println("Connected to " + name)
			r.accept <- connect{conn: conn, name: name}
		}
	}
}

func (r *rm) mainRoutine() {
	for {
		select {
		case conn := <-r.accept:
			client := clientGFD{
				conn: conn.conn,
				name: conn.name,
			}
			if conn.name == "GFD" {
				r.gfd = client
			} else {
				r.clients = append(r.clients, client)
			}
			go r.readRoutine(client)
		case request := <-r.readChan:
			if requestCheck(request.request, "membership") {
				requests := strings.Split(request.request, ",")
				if requests[len(requests)-2] == "add" {
					server := server{
						name:    requests[1],
						deleted: false,
						primary: false,
					}
					if r.detectServer(server.name) {
						if !r.active {
							go r.writeReady(server)
						} else {
							go r.writeCheckpoint(server)
						}
					} else {
						if len(r.servers) == 0 && !r.active {
							server.primary = true
							r.primColor.Println(server.name + " is the primary server")
						}
						add := true
						for _, s := range r.servers {
							if s.name == server.name {
								add = false
							}
						}
						if add {
							r.servers = append(r.servers, server)
						}
						go r.writeReady(server)
					}
				} else if requests[len(requests)-2] == "delete" {
					prim := r.deleteServer(requests[1])
					if !r.active && prim {
						ind := 0
						for ind < len(r.servers) && r.servers[ind].deleted {
							ind += 1
						}
						if ind >= len(r.servers) {
							r.launchServer(requests[1])
							break
						}
						primary := r.servers[ind]
						primary.primary = true
						r.servers[ind] = primary
						go r.writeReady(primary)
						r.primColor.Println(primary.name + " is now the primary server")
					}
					r.launchServer(requests[1])
				}
				r.printServer()
			} else if requestCheck(request.request, "client") {
				serverList := ""
				for _, server := range r.servers {
					if r.active {
						serverList += server.name
						serverList += ";"
					} else if server.primary {
						serverList += server.name
					}
				}
				go r.writeClient(serverList, request.client)
			}
		}
	}
}

func (r *rm) writeCheckpoint(server server) {
	ind := 0
	for ind < len(r.servers) && (r.servers[ind].deleted || r.servers[ind].name == server.name) {
		ind += 1
	}
	if ind >= len(r.servers) {
		go r.writeReady(server)
		return
	}
	requests := "<" + r.servers[ind].name + "," + server.name + ",send_checkpoint>"
	r.gfd.conn.Write([]byte(requests))
	r.sendColor.Println(r.servers[ind].name + " should send checkpoint to " + server.name)
}

func (r *rm) launchServer(name string) {
	requests := "<" + name + ",launch>"
	r.gfd.conn.Write([]byte(requests))
	r.sendColor.Println(name + " is disconnected. Try to reboot the server.")
}

func (r *rm) writeReady(server server) {
	stype := "primary"
	if !server.primary {
		stype = "backup"
	}
	requests := "<" + server.name + "," + r.config + "," + stype + ",ready>"
	r.gfd.conn.Write([]byte(requests))
}

func (r *rm) writeClient(serverList string, client clientGFD) {
	requests := "<" + client.name + "," + r.config + "," + serverList + ",ready>"
	client.conn.Write([]byte(requests))
}

func (r *rm) readRoutine(client clientGFD) {
	reader := bufio.NewReader(client.conn)
	for {
		select {
		default:
			text, err := reader.ReadString('>')
			if err != nil {
				return
			}
			if len(string(text)) != 0 {
				r.readChan <- request{client: client, request: text}
			}
		}
	}
}

func (r *rm) deleteServer(name string) bool {
	for i, server := range r.servers {
		if server.name == name {
			server.deleted = true
			if server.primary {
				server.primary = false
				r.servers[i] = server
				return true
			} else {
				r.servers[i] = server
				return false
			}
		}
	}
	return false
}

func (r *rm) detectServer(name string) bool {
	for i, server := range r.servers {
		if server.name == name && server.deleted {
			r.servers[i].deleted = false
			return true
		}
	}
	return false
}
func (r *rm) printServer() {
	content := ""
	count := 0
	number := len(r.servers)
	if number > 0 {
		content += "("
		for i, server := range r.servers {
			if !server.deleted {
				count += 1
				content += server.name
				if i != number-1 {
					content += ","
				}
			}
		}
		content += ")"
	}
	start := "RM" + ": " + strconv.Itoa(count) + " members"
	start += content
	r.memColor.Println(timestamp() + " " + start)
}

func requestCheck(request, subString string) bool {
	requests := strings.Split(request, ",")
	return strings.Contains(requests[len(requests)-1], subString)
}

func timestamp() string {
	return time.Now().Format("15:04:05")
}
