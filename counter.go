package main

import (
	"bytes"
	"log"
	"net"
	"os"
	"os/exec"
	"time"
	"flag"
	"regexp"
)

const (
	maxDatagramSize = 8192
	mib = 1024 * 1024
)

var console *log.Logger
var logPtr *string
var addressPtr *string
var commandPtr *string
var speedPtr *int
var periodPtr *int

func worker(c <-chan int, ch_b chan<- int) {
	i := 0
	var max float32 = 0
	var min float32 = 0
	heartbeat := time.Tick(time.Duration(*periodPtr) * time.Second)
	for {
		select {
		case quantity := <-c:
			i += quantity * 8;

		case <-heartbeat:
			currentSpeed := float32(i) / float32(*periodPtr) / mib
			if currentSpeed > max {
				max = currentSpeed
			}
			if ( currentSpeed < min || min < float32(*speedPtr) / 1024 ) {
				min = currentSpeed
			}
			if currentSpeed < float32(*speedPtr) / 1024 {
				console.Printf("BAD - %4.2f\n", currentSpeed)
				ch_b <- 1
			} else {
				console.Printf("%4.2f %4.2f %4.2f\n",
					min,
					currentSpeed,
					max)
				ch_b <- 0
			}
			i = 0
		}
	}
}

func beeper(ch_b <-chan int) {
	var buf bytes.Buffer
	state := 0
	for {
		in := <- ch_b
		if state != in {
			console.Println("Incident happens.")
			log.Println("Incident happens.")
			state = in

			r := regexp.MustCompile(`[^\s"']+|"([^"]*)"|'([^']*)'`)
			arr := r.FindAllString(*commandPtr, -1)
			cmd := exec.Command(arr[0])
			cmd.Args = arr
			cmd.Stdout = &buf
			err := cmd.Start()
			if err != nil {
				log.Printf("error: %v\n", err)
			}
			log.Println("Running command and waiting for it to finish...")
			err = cmd.Wait()
			if err == nil {
				log.Println(buf.String())
			} else {
				log.Printf("Command finished with error: %v\n", err)
				console.Printf("Command finished with error: %v\n", err)
			}
		}
	}
}

func main() {
	console = log.New(os.Stdout, "", 0)
	console.Println("Measurement of speed of multicast stream and executes the specified\nprogram when speed is too low.\nhttps://github.com/kk12l (c) 2018")

	console.SetFlags(log.Ldate | log.Ltime)
	logPtr = flag.String("log", os.Args[0] + ".log", "Logging to file.")
	addressPtr = flag.String("address", "235.24.204.1:1234", "Multicast adress.")
	commandPtr = flag.String("command", "cmd /c echo Multicast speed is too low!", "Which command executes when speed of multicast stream is too low.\n")
	speedPtr = flag.Int("speed", 1024, "Minimal speed of multicast stream in KiB.")
	periodPtr = flag.Int("period", 5, "Averaging period for measuring speed in seconds.")
	flag.Parse()

	logFile, err := os.OpenFile(*logPtr, os.O_CREATE | os.O_APPEND | os.O_RDWR, 0666)
	if err != nil {
		log.Panic(err)
	}
	defer logFile.Close()
	log.SetOutput(logFile)

	log.Println("Start.")

	var channel chan int = make(chan int)
	var ch_beep chan int = make(chan int)

	go worker(channel, ch_beep)
	go beeper(ch_beep)

	address := *addressPtr

	log.Printf("Listening on %s\n", address)
	addr, err := net.ResolveUDPAddr("udp", address)
	if err != nil {
		log.Fatal(err)
	}

	// Open up a connection
	conn, err := net.ListenMulticastUDP("udp", nil, addr)
	if err != nil {
		log.Fatal(err)
	}

	conn.SetReadBuffer(maxDatagramSize)

	// Loop forever reading from the socket
	for {
		buffer := make([]byte, maxDatagramSize)
		numBytes, _, err := conn.ReadFromUDP(buffer)
		if err != nil {
			log.Fatal("ReadFromUDP failed:", err)
		}

		channel <- numBytes
	}

	log.Println("Exit.")
}
