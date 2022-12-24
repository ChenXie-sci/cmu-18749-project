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
	"github.com/spf13/viper"
)

const (
	MAX_WRITE_SIZE = 500
	C1_PORT        = 8000
	C2_PORT        = 8001
	C3_PORT        = 8002
	LFD_PORT       = 8003
	R1_PORT        = 8004
	R2_PORT        = 8005
	Active         = "active"
	Passive        = "passive"
	Primary        = "primary"
	Backup         = "backup"
)

type config struct {
	Ip1 string `mapstructure:"server1Ip"`
	Ip2 string `mapstructure:"server2Ip"`
	Ip3 string `mapstructure:"server3Ip"`
}

type server struct {
	acceptClose        chan bool // chan of request accept routine close
	acceptClosed       chan bool // chan of accept routine closed
	mainClose          chan bool // chan of request main routine close
	mainClosed         chan bool // chan of main routine closed
	systemType         string
	replicaType        string
	c1                 net.Listener  // client1 tcp server listener
	c2                 net.Listener  // client2 tcp server listener
	c3                 net.Listener  // client3 tcp server listener
	lfd                net.Listener  // lfd tcp server listener
	r1                 net.Listener  // replica 1 tcp server listener
	r2                 net.Listener  // replica 2 tcp server listener
	accept             chan net.Conn // chan of accepted connection
	requestChan        chan request  // chan of client request
	status             string
	statusChan         chan string
	statusRequest      chan bool   // chan of request active client
	statusResult       chan string // chan of active client results
	connectToChan      chan string
	name               string   // name of the server
	clients            []client // client list for Close
	IP                 string
	checkpointFre      int
	checkpointCount    int
	config             config
	replicas           []client
	checkpointChan     chan bool
	stopCheckpointChan chan bool
	deleteReplicaChan  chan client
	isReady            bool
	requestColor       *color.Color
	receivedColor      *color.Color
	buffer             []string
	oldRequest         string
}

type client struct {
	conn      net.Conn    // client's connection
	response  chan string // chan of response's text
	terminate chan bool   // chan of client's terminate
	port      int
	name      string
}

type request struct {
	client  client // the destionation of the request
	request string // request's text
}

func NewServer() Server {
	s := server{
		accept:             make(chan net.Conn),
		requestChan:        make(chan request),
		acceptClose:        make(chan bool),
		acceptClosed:       make(chan bool),
		mainClose:          make(chan bool),
		mainClosed:         make(chan bool),
		statusChan:         make(chan string),
		statusRequest:      make(chan bool),
		statusResult:       make(chan string),
		status:             "?",
		checkpointChan:     make(chan bool),
		connectToChan:      make(chan string),
		stopCheckpointChan: make(chan bool),
		deleteReplicaChan:  make(chan client),
	}
	return &s
}

func (s *server) Start(name string, ip string, checkpointFreq int) error {
	s.name = name
	s.IP = ip
	s.checkpointFre = checkpointFreq

	// start tcp server
	err := s.tcpStart()
	if err != nil {
		return err
	}
	s.readConfig()
	s.requestColor = color.New(color.FgYellow).Add(color.Bold)
	s.receivedColor = color.New(color.FgRed).Add(color.Bold)

	// run accept and main routine
	go s.acceptRoutine(s.c1)
	go s.acceptRoutine(s.c2)
	go s.acceptRoutine(s.c3)
	go s.acceptRoutine(s.lfd)
	go s.acceptRoutine(s.r1)
	go s.acceptRoutine(s.r2)
	go s.connectionRoutine()
	go s.mainRoutine()

	return nil
}

func (s *server) Close() {
	s.mainClose <- true
	<-s.mainClosed
}

func (s *server) tcpStart() error {
	ls, err := net.Listen("tcp", s.IP+":"+strconv.Itoa(C1_PORT))
	if err != nil {
		return err
	}
	s.c1 = ls

	ls, err = net.Listen("tcp", s.IP+":"+strconv.Itoa(C2_PORT))
	if err != nil {
		return err
	}
	s.c2 = ls

	ls, err = net.Listen("tcp", s.IP+":"+strconv.Itoa(C3_PORT))
	if err != nil {
		return err
	}
	s.c3 = ls

	ls, err = net.Listen("tcp", s.IP+":"+strconv.Itoa(LFD_PORT))
	if err != nil {
		return err
	}
	s.lfd = ls

	ls, err = net.Listen("tcp", s.IP+":"+strconv.Itoa(R1_PORT))
	if err != nil {
		return err
	}
	s.r1 = ls

	ls, err = net.Listen("tcp", s.IP+":"+strconv.Itoa(R2_PORT))
	if err != nil {
		return err
	}
	s.r2 = ls
	return nil
}

func (s *server) readConfig() {
	v := viper.New()
	v.SetConfigFile("config.yaml")
	if err := v.ReadInConfig(); err != nil {
		panic(err)
	}
	serverConfig := config{}
	if err := v.Unmarshal(&serverConfig); err != nil {
		panic(err)
	}
	s.config = serverConfig
}

func (s *server) acceptRoutine(listener net.Listener) {
	for {
		select {
		default:
			conn, err := listener.Accept()
			// fmt.Println("client connect to " + conn.LocalAddr().String())
			if err != nil {
				return
			}
			// conn.
			s.accept <- conn
		}
	}
}

func (s *server) checkpointRoutine() {
	// fmt.Println("start checkpoint routine")
	timer := time.NewTicker(time.Duration(s.checkpointFre * int(time.Second)))
	for {
		select {
		case <-timer.C:
			s.checkpointChan <- true
		case <-s.stopCheckpointChan:
			return
		}
	}
}

func (s *server) connectionRoutine() {
	// fmt.Println("start connection routine")
	for {
		if len(s.replicas) != 2 {
			all := []string{"S1", "S2", "S3"}
			temp := s.name
			for _, replica := range s.replicas {
				temp += "," + replica.name
			}
			for _, name := range all {
				if !strings.Contains(temp, name) {
					s.connectToChan <- name
				}
			}
		} else {
			// fmt.Println("end connection routine")
			return
		}

		time.Sleep(1 * time.Second)
	}
}

func (s *server) mainRoutine() {
	for {
		select {
		case status := <-s.statusChan:
			s.status = status
		case <-s.statusRequest:
			s.statusResult <- s.status
		case conn := <-s.accept:
			client := client{
				conn:      conn,
				terminate: make(chan bool),
				response:  make(chan string, 500),
			}
			s.clients = append(s.clients, client)
			go s.readRoutine(client)
			go s.writeRoutine(client)
		case request := <-s.requestChan:
			if requestCheck(request.request, "request") { // message from client
				// modify request to desired printing format
				if s.isReady && s.replicaType != Backup {
					s.handleRequest(request)
				} else {
					s.buffer = append(s.buffer, request.request)
				}
			} else if requestCheck(request.request, "heartbeat") { // message from heartbeat
				s.printHeartBeat(false, request.request)
				request.client.response <- request.request
			} else if requestCheck(request.request, "ready") { // message from RM
				s.isReady = true
				str := strings.Split(request.request, ",")
				s.replicaType = Primary
				if str[1] == "active" {
					s.systemType = Active
				} else if str[1] == "passive" {
					s.systemType = Passive
					if str[2] == "backup" {
						s.replicaType = Backup
					} else if str[2] == "primary" {
						go s.checkpointRoutine()
					}
				}
				s.printReady()
			} else if requestCheck(request.request, "send_checkpoint") { // message from RM
				requests := strings.Split(request.request, ",")
				receiver := requests[1]
				c, err := s.getReplica(receiver)
				if err != nil {
					color.New(color.FgBlue).Printf("%s is not connected by this server\n", receiver)
					break
				}
				var m []string
				if s.oldRequest != "" {
					oldRequests := strings.Split(s.oldRequest, ",")
					m = []string{s.name, receiver, s.systemType, s.replicaType, oldRequests[2], s.status, "recovery"}
				} else {
					m = []string{s.name, receiver, s.systemType, s.replicaType, "0", s.status, "recovery"}
				}
				message := "<" + strings.Join(m, ",") + ">"
				color.New(color.FgRed).Add(color.Bold).Printf("%s Send %s recovery message with %s\n", timestamp(), receiver, message)
				if err != nil {
					fmt.Println(s.name + " can not connected to " + receiver)
				} else {
					c.response <- message
				}
			} else if requestCheck(request.request, "recovery") { // message from RM
				requests := strings.Split(request.request, ",")
				s.replicaType = Primary
				if requests[2] == "active" {
					s.systemType = Active
				} else {
					s.systemType = Passive
					if requests[3] == "backup" {
						s.replicaType = Backup
					}
				}

				s.processRecovery(request.request)
				index, _ := strconv.Atoi(requests[4])
				for _, re := range s.buffer {
					temp := strings.Split(re, ",")
					tempIndex, _ := strconv.Atoi(temp[2])
					if tempIndex > index {
						s.processRequest(re)
					}
				}
				s.isReady = true
				s.printReady()
			} else if requestCheck(request.request, "checkpoint") { // message from primary replica
				if s.isReady {
					s.printCheckpoint(false, request.request)
					s.printStatus(true, request.request)
					requests := strings.Split(request.request, ",")
					s.status = requests[3]
					s.checkpointCount, _ = strconv.Atoi(requests[2])
					s.printStatus(false, request.request)
				}
			}

		case <-s.mainClose:
			s.mainClosed <- true
			s.c1.Close()
			s.c2.Close()
			s.c3.Close()
			s.lfd.Close()
			for _, client := range s.clients {
				client.conn.Close()
			}
			return
		case <-s.checkpointChan:
			s.checkpointCount += 1
			checkpointCount := strconv.Itoa(s.checkpointCount)
			for _, replica := range s.replicas {

				m := []string{s.name, replica.name, checkpointCount, s.status, "checkpoint"}
				message := "<" + strings.Join(m, ",") + ">"
				s.printCheckpoint(true, message)
				replica.response <- message
			}
		case server := <-s.connectToChan:
			s.connectToServer(server)
		case server := <-s.deleteReplicaChan:
			index := -1
			for i, replica := range s.replicas {
				if replica.conn == server.conn {
					index = i
					s.receivedColor.Println(timestamp() + " " + replica.name + " is disconnected")
					go s.connectionRoutine()
				}
			}
			if index != -1 {
				if len(s.replicas) == 1 {
					s.replicas = nil
				} else {
					if index == 0 {
						s.replicas = s.replicas[1:]
					} else {
						s.replicas = []client{s.replicas[0]}
					}
				}
			}
		}
	}
}

func (s *server) readRoutine(c client) {
	reader := bufio.NewReader(c.conn)
	for {
		select {
		default:
			text, err := reader.ReadString('>')
			// if err == io.EOF {
			// fmt.Print(text)
			if err != nil {

				s.deleteReplicaChan <- c
				close(c.response)

				return
			}
			if len(string(text)) != 0 {
				s.requestChan <- request{client: c, request: text}
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

			if requestCheck(response, "reply") {
				s.printRequest(false, string(response))
				client.conn.Write([]byte(response))
			} else if requestCheck(response, "heartbeat") {
				s.printHeartBeat(true, response)
				requests := strings.Split(response, ",")
				requests[0] = "<" + requests[0][1:]
				requests[1] = s.name
				// fmt.Println(strings.Join(requests, ","))
				client.conn.Write([]byte(strings.Join(requests, ",")))
			} else if requestCheck(response, "checkpoint") {
				client.conn.Write([]byte(response))
			} else if requestCheck(response, "recovery") {

				client.conn.Write([]byte(response))
			}
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
	s.requestColor.Println(timestamp() + " " + "my_state_" + s.name + " = " + s.status + " " + action + " processing " + request)
}

func (s *server) printHeartBeat(send bool, request string) {
	requests := strings.Split(request, ",")
	action := "sends heartbeat to " + requests[1]
	if !send {
		action = "receives heartbeat from " + requests[0][1:]
	}

	fmt.Println(timestamp() + " " + requests[2] + " " + s.name + " " + action)
}

func (s *server) printCheckpoint(send bool, request string) {
	requests := strings.Split(request, ",")
	action := "sends checkpoint to " + requests[1]
	if !send {
		action = "receives checkpoint from " + requests[0][1:]
	}
	fmt.Println(timestamp() + " " + requests[2] + " my_state=" + requests[3] + " " + s.name + " " + action)
}

func (s *server) getReplicaName(port int) string {
	server_number := 1
	switch s.name {
	case "S1":
		if port == R1_PORT {
			server_number = 2
		} else if port == R2_PORT {
			server_number = 3
		}
	case "S2":
		if port == R1_PORT {
			server_number = 1
		} else if port == R2_PORT {
			server_number = 3
		}
	case "S3":
		if port == R1_PORT {
			server_number = 1
		} else if port == R2_PORT {
			server_number = 2
		}
	}
	return "S" + strconv.Itoa(server_number)
}

func (s *server) getPort(conn net.Conn) int {
	port, _ := strconv.Atoi(strings.Split(conn.LocalAddr().String(), ":")[1])
	fmt.Println(conn.LocalAddr().String())
	return port
}

func (s *server) connectToServer(server string) (client, error) {
	if server == s.name {
		return client{}, errors.New("can not")
	}

	ip := ""
	port := 0

	switch s.name {
	case "S1":
		if server == "S2" {
			port = R1_PORT
			ip = s.config.Ip2
		} else if server == "S3" {
			port = R2_PORT
			ip = s.config.Ip3
		}

	case "S2":
		if server == "S1" {
			port = R1_PORT
			ip = s.config.Ip1
		} else if server == "S3" {
			port = R2_PORT
			ip = s.config.Ip3
		}
	case "S3":
		if server == "S1" {
			port = R1_PORT
			ip = s.config.Ip1
		} else if server == "S2" {
			port = R2_PORT
			ip = s.config.Ip2
		}
	}

	d := net.Dialer{Timeout: time.Second}
	conn, err := d.Dial("tcp", ip+":"+strconv.Itoa(port))
	if err != nil {
		return client{}, errors.New("can not")
	}

	client := client{
		conn:      conn,
		terminate: make(chan bool),
		response:  make(chan string, 500),
		port:      port,
		name:      server,
	}
	s.replicas = append(s.replicas, client)

	go s.readRoutine(client)
	go s.writeRoutine(client)
	s.receivedColor.Printf("%s Connect to %s with port %d\n", timestamp(), server, port)
	return client, nil

}

func (s *server) handleRequest(request request) {
	s.oldRequest = request.request
	requests := strings.Split(request.request, ",")
	requests = append(requests[:3], requests[4:]...)
	printing_request := strings.Join(requests, ",")
	s.printRequest(true, printing_request)

	request.client.response <- s.processRequest(request.request)
}

func (s *server) processRequest(request string) string {

	requests := strings.Split(request, ",")
	status := requests[3]
	requests = append(requests[:3], requests[4:]...)
	printing_request := strings.Join(requests, ",")
	s.printStatus(true, printing_request)
	s.status = status
	s.printStatus(false, printing_request)

	requests[len(requests)-1] = "reply>"
	return strings.Join(requests, ",")
}

func (s *server) getReplica(server string) (client, error) {

	for _, replica := range s.replicas {
		if replica.name == server {
			return replica, nil
		}
	}
	return client{}, errors.New("123")
}

func (s *server) processRecovery(recovery string) {
	requests := strings.Split(recovery, ",")
	status := requests[5]

	s.printRecovery(recovery)
	s.printStatus(true, recovery)
	s.status = status
	s.printStatus(false, recovery)
}

func (s *server) printRecovery(recovery string) {
	action := "Received"
	s.receivedColor.Println(timestamp() + " " + action + " " + recovery)
}

func (s *server) printReady() {
	s.receivedColor.Printf("%s %s is ready with %s system as %s server\n", timestamp(), s.name, s.systemType, s.replicaType)
}

func requestCheck(request, subString string) bool {
	requests := strings.Split(request, ",")
	return strings.Contains(requests[len(requests)-1], subString)
}

func timestamp() string {
	return time.Now().Format("15:04:05")
}
