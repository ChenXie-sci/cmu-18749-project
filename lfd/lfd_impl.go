package main

import (
	"bufio"
	"fmt"
	"net"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/fatih/color"
)

const (
	MAX_WRITE_SIZE = 500
	GFD_NAME       = "GFD"
)

type lfd struct {
	acceptClose      chan bool     // chan of request accept routine close
	acceptClosed     chan bool     // chan of accept routine closed
	mainClose        chan bool     // chan of request main routine close
	mainClosed       chan bool     // chan of main routine closed
	acceptServer     chan net.Conn // chan of accepted server connection
	acceptGFD        chan net.Conn // chan of accepted gfd connection
	heartbeatChan    chan heartbeat
	launchServerChan chan bool
	sendServer       chan string
	name             string // name of the lfd
	serverName       string
	server           serverGFD
	gfd              serverGFD // client list for Close
	heartbeatPassed  chan bool
	closeHeartbeat   chan bool
	heartbeatFreq    int
	serverPort       string
	gfdPort          string
	serverCreated    bool
	addColor         *color.Color
	delColor         *color.Color
}

type serverGFD struct {
	conn              net.Conn    // server's connection
	response          chan string // chan of response's text
	terminate         chan bool   // chan of server's terminate
	heartbeatSent     int
	heartbeatReceived int
	name              string
	deleted           bool
}

type heartbeat struct {
	heartbeat string
	server    bool //true if server, false if gfd
}

func NewLFD() LFD {
	l := lfd{
		acceptServer:     make(chan net.Conn),
		acceptGFD:        make(chan net.Conn),
		heartbeatChan:    make(chan heartbeat),
		sendServer:       make(chan string),
		acceptClose:      make(chan bool),
		acceptClosed:     make(chan bool),
		mainClose:        make(chan bool),
		mainClosed:       make(chan bool),
		heartbeatPassed:  make(chan bool),
		closeHeartbeat:   make(chan bool),
		launchServerChan: make(chan bool),
		heartbeatFreq:    0,
		serverPort:       "0",
		gfdPort:          "0",
		serverCreated:    false,
	}
	return &l
}

func (l *lfd) Start(name string, freq int, server_name string, server_port string, gfd_port string) error {
	l.name = name
	l.heartbeatFreq = freq
	l.serverPort = server_port
	l.gfdPort = gfd_port
	l.serverName = server_name
	// start tcp server
	l.addColor = color.New(color.FgBlue).Add(color.Bold)
	l.delColor = color.New(color.FgRed).Add(color.Bold)
	go l.launchServer()
	go l.mainRoutine(server_name)
	go l.heartBeatCounter()
	go l.serverStart()
	go l.gfdStart()

	// run accept and main routine
	//go l.acceptRoutine(l.s1)
	return nil
}

func (l *lfd) Close() {
	l.mainClose <- true
	l.closeHeartbeat <- true
	<-l.mainClosed
}

func (l *lfd) serverStart() error {
	fmt.Printf("Connecting to %s... for server\n", l.serverPort)
	for {
		conn, err := net.Dial("tcp", l.serverPort)
		if err == nil {
			l.acceptServer <- conn
			break
		}
	}
	return nil
}

func (l *lfd) gfdStart() error {
	for {
		fmt.Printf("Connecting to %s... for gfd\n", l.gfdPort)
		conn, err := net.Dial("tcp", l.gfdPort)
		if err == nil {
			l.acceptGFD <- conn
			fmt.Printf("Connected to %s... for gfd\n", l.gfdPort)
			return nil
		}
		time.Sleep(time.Second * 5)
	}

}

func (l *lfd) heartBeatCounter() {
	seconds := l.heartbeatFreq
	secs := (time.Duration(seconds) * time.Second)
	ticker := time.NewTicker(secs)
	check := false
	for _ = range ticker.C {
		select {
		case <-l.closeHeartbeat:
			ticker.Stop()
			check = true
			break
		default:
			//If no signal to stop has been received,
			//it signals the main routine by telling it
			//an epoch has past.
			l.heartbeatPassed <- true
		}
		if check == true {
			break
		}
	}
	return
}

func (l *lfd) mainRoutine(server_name string) {
	for {
		select {
		case conn := <-l.acceptServer:
			server := serverGFD{
				conn:              conn,
				terminate:         make(chan bool),
				response:          make(chan string, 500),
				heartbeatSent:     -1,
				heartbeatReceived: -1,
				name:              server_name,
				deleted:           false,
			}
			l.server = server
			l.serverCreated = true
			go l.readRoutine(true)
		case conn := <-l.acceptGFD:
			gfd := serverGFD{
				conn:              conn,
				terminate:         make(chan bool),
				response:          make(chan string, 500),
				heartbeatSent:     -1,
				heartbeatReceived: -1,
				name:              GFD_NAME,
				deleted:           false,
			}
			l.gfd = gfd
			go l.readRoutine(false)
		case <-l.heartbeatPassed:
			if l.serverCreated {
				go l.writeRoutine(true)
			}
		case sendInfo := <-l.sendServer:
			if l.serverCreated {
				go l.sendRoutine(sendInfo)
			}
		case heartbeat := <-l.heartbeatChan:
			if l.heartbeatCheck(heartbeat) {
				l.printHeartBeat(false, heartbeat.server)
				if !heartbeat.server {
					go l.writeRoutine(false)
				}
			} else {
				fmt.Println(timestamp() + " Server responded with incorrect heartbeat")
			}
		case <-l.launchServerChan:
			go l.launchServer()
		case <-l.mainClose:
			l.mainClosed <- true
			l.server.conn.Close()
			return
		}
	}
}

func (l *lfd) readRoutine(server bool) {
	item := l.server
	if !server {
		item = l.gfd
	}
	reader := bufio.NewReader(item.conn)
	for {
		select {
		default:
			text, err := reader.ReadString('>')
			// if err == io.EOF {
			if err != nil {
				if !server {
					fmt.Println("gfd is disconnected")
					l.gfdStart()
				}
				close(item.response)
				return
			}
			requests := strings.Split(text, ",")
			if strings.Contains(requests[len(requests)-1], "heartbeat") {
				l.heartbeatChan <- heartbeat{heartbeat: text, server: server}
			} else if strings.Contains(requests[len(requests)-1], "launch") {
				l.launchServerChan <- true
			} else {
				l.sendServer <- text
			}
		}
	}
}
func (l *lfd) sendRoutine(sendInfo string) {
	s := l.server
	s.conn.Write([]byte(sendInfo))
}

func (l *lfd) writeRoutine(server bool) {
	if server {
		s := l.server
		if s.heartbeatSent != s.heartbeatReceived {
			l.printServerFailed(s.name, s.heartbeatSent)
			l.serverCreated = false
			l.sendMembershp(false)
			go l.serverStart()
			fmt.Println(timestamp() + ": retrying connecting to server")
		}
		l.server.heartbeatSent += 1
		l.printHeartBeat(true, true)
		requests := "<" + l.name + "," + s.name + "," + strconv.Itoa(s.heartbeatSent+1) + ",heartbeat>"
		s.conn.Write([]byte(requests))
	} else {
		g := l.gfd
		l.gfd.heartbeatSent += 1
		l.printHeartBeat(true, false)
		requests := "<" + g.name + "," + l.name + "," + strconv.Itoa(g.heartbeatSent+1) + ",heartbeat>"
		g.conn.Write([]byte(requests))
	}
}

func (l *lfd) printServerFailed(serverName string, num int) {
	fmt.Println(timestamp() + "  Server {" + serverName + "} failed to respond after heartbeat #" + strconv.Itoa(num) + " was sent")
}

func (l *lfd) printHeartBeat(send bool, server bool) {
	item := l.server
	if !server {
		item = l.gfd
	}
	action := "sending heartbeat to "
	count := item.heartbeatSent
	if !send {
		count = item.heartbeatReceived
		action = "receives heartbeat from "
	}
	fmt.Println(timestamp() + " " + strconv.Itoa(count) + ", " + l.name + " " + action + item.name)
}

func (l *lfd) heartbeatCheck(heartbeat heartbeat) bool {
	item := l.server
	if !heartbeat.server {
		item = l.gfd
	}
	requests := strings.Split(heartbeat.heartbeat, ",")
	sent := item.heartbeatSent
	correct := false
	if heartbeat.server {
		correct = strings.Contains(requests[len(requests)-1], "heartbeat") && strings.Contains(requests[2], strconv.Itoa(sent))
		if correct {
			l.server.heartbeatReceived += 1
			if sent == 0 {
				l.sendMembershp(true)
			}
		}
	} else {
		correct = strings.Contains(requests[len(requests)-1], "heartbeat") && strings.Contains(requests[2], strconv.Itoa(sent+1))
		if correct {
			l.gfd.heartbeatReceived += 1
		}
	}
	return correct
}

func (l *lfd) sendMembershp(add bool) {
	action := ": add replica "
	added := "add"
	color := l.addColor
	if !add {
		action = ": delete replica "
		added = "delete"
		l.server.deleted = true
		color = l.delColor
	}
	requests := "<" + l.name + "," + l.server.name + "," + added + ",membership>"
	color.Println(timestamp() + " " + l.name + action + l.server.name)
	l.gfd.conn.Write([]byte(requests))
}

func (l *lfd) launchServer() error {
	prg := "run_server.sh"
	arg1 := l.serverName
	arg2 := strings.Split(l.serverPort, ":")[0]
	arg3 := l.heartbeatFreq
	cmd := exec.Command("/bin/bash", prg, arg1, arg2, strconv.Itoa(arg3))
	// cmd := exec.Command("pwd")
	err := cmd.Run()
	// out, err := exec.Command("ls", "-l").Output()
	if err != nil {
		fmt.Println(err.Error())
		return err
	}
	select {}

}

func timestamp() string {
	return time.Now().Format("15:04:05")
}
