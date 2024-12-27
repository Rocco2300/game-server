package main

import (
	"encoding/json"
	"log"
	"net"
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
	data.Username = "Rocco"
	data.RoomId = "asdfasdfasdf"
	data.Hosting = true

	buf, err := json.Marshal(data)
	if err != nil {
		log.Fatal(err)
	}

	conn.Write(buf)
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
