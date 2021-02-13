package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"golang.org/x/net/ipv4"
	"net"
	"net/http"
	"os"
	"strconv"
)

// From https://github.com/grahamking/latency/blob/master/tcp.go
type TCPHeader struct {
	Source      uint16
	Destination uint16
	SeqNum      uint32
	AckNum      uint32
	DataOffset  uint8
	Reserved    uint8
	ECN         uint8
	Ctrl        uint8
	Window      uint16
	Checksum    uint16
	Urgent      uint16
	Options     []TCPOption
}

type TCPOption struct {
	Kind   uint8
	Length uint8
	Data   []byte
}

func NewTCPHeader(data []byte) *TCPHeader {
	var tcp TCPHeader
	r := bytes.NewReader(data)
	binary.Read(r, binary.BigEndian, &tcp.Source)
	binary.Read(r, binary.BigEndian, &tcp.Destination)
	binary.Read(r, binary.BigEndian, &tcp.SeqNum)
	binary.Read(r, binary.BigEndian, &tcp.AckNum)

	var mix uint16
	binary.Read(r, binary.BigEndian, &mix)
	tcp.DataOffset = byte(mix >> 12)  // top 4 bits
	tcp.Reserved = byte(mix >> 9 & 7) // 3 bits
	tcp.ECN = byte(mix >> 6 & 7)      // 3 bits
	tcp.Ctrl = byte(mix & 0x3f)       // bottom 6 bits

	binary.Read(r, binary.BigEndian, &tcp.Window)
	binary.Read(r, binary.BigEndian, &tcp.Checksum)
	binary.Read(r, binary.BigEndian, &tcp.Urgent)

	return &tcp
}
// End from https://github.com/grahamking/latency/blob/master/tcp.go

type contextKey struct {
	key string
}

var ConnContextKey = &contextKey{"http-conn"}

func SaveConnInContext(ctx context.Context, c net.Conn) context.Context {
	return context.WithValue(ctx, ConnContextKey, c)
}

func GetConnFromHttpRequest(r *http.Request) net.Conn {
	return r.Context().Value(ConnContextKey).(net.Conn)
}

type RequestInfo struct {
	addr string
	ttl  int
}

func logUserRequest(content string) {
	f, err := os.OpenFile("./user-request.log", os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0600)
	if err != nil {
		panic(err)
	}

	defer f.Close()

	if _, err = f.WriteString(content + "\n"); err != nil {
		panic(err)
	}
}

func main() {

	infoChan := make(chan RequestInfo)
	requestInfoBuffer := make(map[string]int)

	// Capture Packet with TTL
	captureListenerAddr, _ := net.ResolveIPAddr("ip4", "0.0.0.0")
	ip4TcpListener, _ := net.ListenIP("ip4:tcp", captureListenerAddr)
	ipConn, _ := ipv4.NewRawConn(ip4TcpListener)

	go func() {
		for {
			buf := make([]byte, 1480)
			header, payload, _, _ := ipConn.ReadFrom(buf)
			tcpHeader := NewTCPHeader(payload)
			if tcpHeader.Destination == 8089 {
				requestInfo := RequestInfo{
					addr: header.Src.String() + ":" + strconv.Itoa(int(tcpHeader.Source)),
					ttl:  header.TTL,
				}
				go logUserRequest(requestInfo.addr + "|" + strconv.Itoa(requestInfo.ttl))

				// Tell http goroutine about all request addr mappings
				fmt.Println("Pushed to buffer & chan:", requestInfo)
				requestInfoBuffer[requestInfo.addr] = requestInfo.ttl
				infoChan <- requestInfo
			}
		}
	}()

	// Start socket server
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {

		var userTTL int

		conn := GetConnFromHttpRequest(r)
		fmt.Println(conn.RemoteAddr())

		waitForCorrectUserRequestInfo := make(chan bool)

		// If either of a) and b) has got a correct userRequestInfo, then reply to user
		// a) wait from channel
		go func() {
			for {
				pkt := <-infoChan
				fmt.Println("Received from Wait chnl:", pkt)
				if pkt.addr == conn.RemoteAddr().String() {
					fmt.Println("Matched!")
					userTTL = pkt.ttl
					waitForCorrectUserRequestInfo <- true
					break
				}
			}
		}()

		// b) or test from the existed map buffer
		go func() {
			//fmt.Println("Test from buffer:", requestInfoBuffer)
			for {
				for addr, ttl := range requestInfoBuffer {
					//fmt.Println("Test from buffer: ", rInfo)
					if addr == conn.RemoteAddr().String() {
						fmt.Println("Matched!")
						userTTL = ttl
						waitForCorrectUserRequestInfo <- true
						break
					}
				}
			}
		}()

		<-waitForCorrectUserRequestInfo

		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")

		visitedRouters := 0

		rpCode := "祝你牛气冲天0209"

		if userTTL < 64 {
			visitedRouters = 64 - userTTL
		} else if userTTL < 128 {
			visitedRouters = 128 - userTTL
		} else {
			visitedRouters = 256 - userTTL
		}

		if userTTL > 200 { // Got red packet
			w.Write([]byte(`{"visited_routers":` + strconv.Itoa(visitedRouters) + `,"remaining_routers":` + strconv.Itoa(userTTL) + `,"rpCode":"` + rpCode + `"}`))
		} else {
			w.Write([]byte(`{"visited_routers":` + strconv.Itoa(visitedRouters) + `,"remaining_routers":` + strconv.Itoa(userTTL) + `,"rpCode":null}`))
		}
	})

	server := http.Server{
		Addr:        ":8089",
		ConnContext: SaveConnInContext,
	}

	go server.ListenAndServe()

	select {}

}
