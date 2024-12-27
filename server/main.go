package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"sync"
)

type ConnectionRequest struct {
	Username string `json:"username"`
	Hosting  bool   `json:"hosting"`
	RoomId   string `json:"roomId"`
}

type ConnectionResponse struct {
	Success      bool   `json:"success"`
	ErrorMessage string `json:"errorMessage"`
}

type PlayerConn struct {
	Username string
	Addr     net.Addr
}

type Server struct {
	Rooms       *sync.Map
	Connections *sync.Map
}

var server = Server{}

func main() {
	addr := net.UDPAddr{
		IP:   net.IPv4(0, 0, 0, 0),
		Port: 12345,
		Zone: "",
	}
	listener, err := net.ListenUDP("udp", &addr)
	if err != nil {
		log.Fatal(err)
	}
	defer listener.Close()

	server.Rooms = new(sync.Map)
	server.Connections = new(sync.Map)
	for {
		buf := make([]byte, 1024)
		n, raddr, err := listener.ReadFrom(buf)
		if err != nil {
			continue
		}

		request := ConnectionRequest{}
		err = json.Unmarshal(buf[:n], &request)
		if err != nil {
			log.Println(err)
			continue
		}

		response := ConnectionResponse{}
		if _, exists := server.Connections.Load(request.Username); !exists {
			response, err = handleConnectionRequest(request, raddr)

			if err != nil {
				log.Println(err)
				log.Println("error in resolving request")
			}
		}

		buf, err = json.Marshal(response)
		if err != nil {
			log.Println(err)
			log.Println("couldn't marshal response data to string. \n", err)
		}

		_, err = listener.WriteTo(buf, raddr)
		if err != nil {
			server.Connections.Delete(request.Username)
			log.Println("could not respond to user...")
		}
	}
}

func handleConnectionRequest(request ConnectionRequest, raddr net.Addr) (ConnectionResponse, error) {
	response, err := handleConnectionResponse(request, raddr)
	if err != nil {
		log.Println(err)

		errBuf := fmt.Sprintf("failed to resolve player", request.Username, "connection")
		return response, errors.New(errBuf)
	}

	return response, nil
}

func handleConnectionResponse(request ConnectionRequest, raddr net.Addr) (ConnectionResponse, error) {
	response := ConnectionResponse{}
	_, exists := server.Rooms.Load(request.RoomId)
	if request.Hosting && !exists {
		response.Success = true
		server.Rooms.Store(request.RoomId, []string{request.Username})
	} else if request.Hosting && exists {
		response.Success = false
		response.ErrorMessage = "room already exists"

		log.Println("couldn't connect ", request.Username)
		return response, errors.New("room already exists")
	} else if !request.Hosting && exists {
		response.Success = true
	} else if !request.Hosting && !exists {
		response.Success = false
		response.ErrorMessage = "room doesn't exist"

		log.Println("couldn't connect ", request.Username)
		return response, errors.New("room doesn't exist")
	}

	playerConn := PlayerConn{}
	playerConn.Username = request.Username
	playerConn.Addr = raddr
	server.Connections.Store(request.Username, playerConn)

	log.Println(request.Username, "connected.")
	return response, nil
}
