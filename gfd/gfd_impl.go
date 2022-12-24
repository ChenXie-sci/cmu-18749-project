package main

import (
	"bufio"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/fatih/color"
)

const (
	MAX_WRITE_SIZE = 500
	L1_PORT        = 8000
	L2_PORT        = 8001
	L3_PORT        = 8002
	RM_PORT        = 8003
	LIMIT          = 5
)

var (
	NAME = ""
)

type server struct {
	acceptClose  chan bool // chan of request accept routine close
	acceptClosed chan bool // chan of accept routine closed
	mainClose    chan bool // chan of request main routine close
	mainClosed   chan bool // chan of main routine closed

	lfd1               net.Listener  // lfd1 tcp server listener
	lfd2               net.Listener  // lfd2 tcp server listener
	lfd3               net.Listener  // lfd3 tcp server listener
	accept             chan lfd_conn // chan of accepted connection
	readChan           chan request  // chan of client request
	status             string
	statusChan         chan string
	statusRequest      chan bool   // chan of request active client
	statusResult       chan string // chan of active client results
	name               string      // name of the server
	clients            []*client   // client list for Close
	rm                 *client
	servers            []string     // connected server list
	timer              *time.Ticker // timer for request heartbeat
	writeCloseChan     chan client  // chan of lfd server is closed
	writeCloseDoneChan chan bool    // chan of lfd server is closed
	IP                 string
	memColor           *color.Color
	rmIsConnected      bool
	rmCloseChan        chan bool
}

type lfd_conn struct {
	conn net.Conn // lfd's connection
	name string   // lfd name
}

type client struct {
	conn            net.Conn    // lfd's connection
	name            string      // lfd name
	response        chan string // chan of response's text
	terminate       chan bool   // chan of client's terminate
	heartbeat_count int         // heartbear_count
	closed          bool
}

type request struct {
	client  client // the destionation of the request
	request string // request's text
}

func NewServer() Server {
	s := server{
		accept:             make(chan lfd_conn),
		readChan:           make(chan request, 10000),
		acceptClose:        make(chan bool),
		acceptClosed:       make(chan bool),
		mainClose:          make(chan bool),
		mainClosed:         make(chan bool),
		statusChan:         make(chan string),
		statusRequest:      make(chan bool),
		statusResult:       make(chan string),
		writeCloseChan:     make(chan client, 1),
		writeCloseDoneChan: make(chan bool),
		status:             "?",
		rmCloseChan:        make(chan bool),
	}
	return &s
}

func (s *server) Start(name string, heartbeat int, ip string) error {
	s.timer = time.NewTicker(time.Duration(heartbeat * int(time.Second)))
	s.IP = ip
	s.name = name
	NAME = name
	s.memColor = color.New(color.FgRed).Add(color.Bold)

	// start tcp server
	err := s.tcpStart()
	if err != nil {
		return err
	}
	fmt.Println("Started GFD with heartbeat_freq: " + strconv.Itoa(heartbeat))
	// print server
	s.printServer()

	// run accept and main routine
	go s.acceptRoutine(s.lfd1, "L1")
	go s.acceptRoutine(s.lfd2, "L2")
	go s.acceptRoutine(s.lfd3, "L3")
	go s.mainRoutine()

	time.Sleep(time.Second * 1)

	conn, err := net.Dial("tcp", s.IP+":"+strconv.Itoa(RM_PORT))
	if err == nil {
		fmt.Println("Connected to RM")
		s.accept <- lfd_conn{conn: conn, name: "RM"}
	} else {
		s.mainClose <- true
		// fmt.Println("Cannot connect to RM!!!!!!!")
		fmt.Println(err)
		return errors.New("cannot connect to rm")
	}

	return nil
}

func (s *server) Close() {
	s.mainClose <- true
	<-s.mainClosed
	s.lfd1.Close()
	s.lfd2.Close()
	s.lfd3.Close()
}

func (s *server) tcpStart() error {
	ls, err := net.Listen("tcp", s.IP+":"+strconv.Itoa(L1_PORT))
	if err != nil {
		return err
	}
	s.lfd1 = ls

	ls, err = net.Listen("tcp", s.IP+":"+strconv.Itoa(L2_PORT))
	if err != nil {
		return err
	}
	s.lfd2 = ls

	ls, err = net.Listen("tcp", s.IP+":"+strconv.Itoa(L3_PORT))
	if err != nil {
		return err
	}
	s.lfd3 = ls

	return nil
}

func (s *server) acceptRoutine(listener net.Listener, name string) {
	for {
		select {
		default:
			conn, err := listener.Accept()
			// fmt.Println("client connect to " + conn.LocalAddr().String())
			if err != nil {
				return
			}
			s.accept <- lfd_conn{conn: conn, name: name}
		}
	}
}

func (s *server) mainRoutine() {
	for {
		select {
		case <-s.rmCloseChan:
			s.rmIsConnected = false
		case name := <-s.writeCloseChan:
			s.deleteClient(name)
			s.writeCloseDoneChan <- true
		case <-s.timer.C:
			for _, client := range s.clients {
				heartbeat_str := client.generateHeartBeat()
				//fmt.Println(heartbeat_str)
				s.printHeartBeat(true, heartbeat_str)
				// if client.response
				client.response <- heartbeat_str
			}
		case status := <-s.statusChan:
			s.status = status
		case <-s.statusRequest:
			s.statusResult <- s.status
		case conn := <-s.accept:
			client := client{
				conn:            conn.conn,
				name:            conn.name,
				terminate:       make(chan bool),
				response:        make(chan string, 500),
				heartbeat_count: 0,
				closed:          false,
			}
			if client.name == "RM" {
				s.rm = &client
				s.rmIsConnected = true
			} else {
				s.clients = append(s.clients, &client)
			}

			go s.readRoutine(client)
			go s.writeRoutine(client)
		case request := <-s.readChan:
			//fmt.Println(request.request)
			if requestCheck(request.request, "membership") {
				if s.rmIsConnected {
					requests := strings.Split(request.request, ",")
					server := requests[1]
					if requests[len(requests)-2] == "add" {
						if s.detectServer(server) {
							fmt.Println("There is already same server name in the connection list")
						} else {
							s.servers = append(s.servers, server)
						}
					} else if requests[len(requests)-2] == "delete" {
						s.deleteServer(server)
					}
					s.printServer()
					s.rm.response <- request.request
				} else {
					s.readChan <- request
				}

			} else if requestCheck(request.request, "heartbeat") {
				s.printHeartBeat(false, request.request)
			} else if requestCheck(request.request, "launch") || requestCheck(request.request, "ready") || requestCheck(request.request, "send_checkpoint") { // should send to correponding server
				fmt.Println("receive message of " + request.request + " from RM and retransmit to corresponding LFD server")
				message := strings.Split(request.request, ",")
				server := message[0][1:]
				number := server[1:]
				for _, client := range s.clients {
					if strings.Contains(client.name, number) {
						client.response <- request.request
						break
					}
				}
			}

		case <-s.mainClose:
			s.mainClosed <- true
			s.lfd1.Close()
			s.lfd2.Close()
			s.lfd3.Close()
			s.rm.conn.Close()
			for _, client := range s.clients {
				client.conn.Close()
			}
			return
		}
	}
}

func (s *server) readRoutine(client client) {
	reader := bufio.NewReader(client.conn)
	for {
		select {
		default:
			text, err := reader.ReadString('>')
			if err != nil {
				if client.name == "RM" {
					s.rmCloseChan <- true
					fmt.Println("rm is close")
					s.reconnectRM()
				}
				s.writeCloseChan <- client
				<-s.writeCloseDoneChan
				close(client.response)
				return
			}
			if len(string(text)) != 0 {
				s.readChan <- request{client: client, request: text}
			}

		}
	}
}

func (s *server) writeRoutine(client client) {
	for {
		select {
		case response, ok := <-client.response:
			if !ok {
				return
			}

			client.conn.Write([]byte(response))
		}
	}
}

func (s *server) printRequest(received bool, request string) {
	action := "Received"
	if !received {
		action = "Sending"
	}

	fmt.Println(timestamp() + " " + action + " " + request)
}

func (s *server) printStatus(before bool, request string) {
	action := "before"
	if !before {
		action = "after"
	}
	fmt.Println(timestamp() + " " + "my_state_" + s.name + " = " + s.status + " " + action + " processing " + request)
}

func (s *server) printHeartBeat(send bool, request string) {
	action := "sends heartbeat to "
	if !send {
		action = "receives heartbeat from "
	}
	requests := strings.Split(request, ",")
	// fmt.Print(requests)
	fmt.Println(timestamp() + " " + requests[2] + " " + s.name + " " + action + requests[1])
}

func (c *client) generateHeartBeat() string {
	content := "<" + NAME + "," + c.name + "," + strconv.Itoa(c.heartbeat_count) + ",heartbeat>"
	c.heartbeat_count += 1
	return content
}

func (s *server) printServer() {
	content := NAME + ": " + strconv.Itoa(len(s.servers)) + " members"
	number := len(s.servers)
	if number > 0 {
		content += "("
		content += strings.Join(s.servers, ", ")
		content += ")"
	}
	s.memColor.Println(timestamp() + " " + content)
}

func (s *server) deleteServer(name string) {
	for i, s_element := range s.servers {
		if s_element == name {
			s.servers = append(s.servers[:i], s.servers[i+1:]...)
			return
		}
	}
}

func (s *server) detectServer(name string) bool {
	for _, s_element := range s.servers {
		if s_element == name {
			return true
		}
	}
	return false
}

func (s *server) deleteClient(c client) {
	// if slices.Contains(s.clients, c) {
	for i, client := range s.clients {
		if client.name == c.name {
			s.clients = append(s.clients[:i], s.clients[i+1:]...)
			return
		}
	}
}
func (s *server) reconnectRM() {
	for {
		conn, err := net.Dial("tcp", s.IP+":"+strconv.Itoa(RM_PORT))
		if err == nil {
			fmt.Println("Connected to RM")
			s.accept <- lfd_conn{conn: conn, name: "RM"}
			break
		}
	}
}

func requestCheck(request, subString string) bool {
	requests := strings.Split(request, ",")
	return strings.Contains(requests[len(requests)-1], subString)
}

func timestamp() string {
	return time.Now().Format("15:04:05")
}
