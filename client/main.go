package main

import (
	"encoding/json"
	"log"
	"net"
	"os"
)

type PlayerData struct {
	Username string `json:"username"`
	Hosting  bool   `json:"hosting"`
	RoomId   string `json:"roomId"`
}

func main() {
	conn, err := net.Dial("udp", "localhost:12345")
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	data := PlayerData{}
	data.Username = os.Args[1]
	data.RoomId = os.Args[2]
	if os.Args[3] == "-h" {
		data.Hosting = true
	} else {
		data.Hosting = false
	}

	buf, err := json.Marshal(data)
	if err != nil {
		log.Fatal(err)
	}

	n, err := conn.Write(buf)
	if err != nil {
		log.Println(err)
		return
	}
	log.Println("wrote", n, "bytes.")

	for {
		buffer := make([]byte, 4096)
		n, err := conn.Read(buffer)
		if err != nil {
			log.Fatal(err)
		}

		status := string(buffer[:n])
		log.Println(status)
	}
}
