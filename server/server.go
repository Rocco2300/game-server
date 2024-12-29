package server

import (
	"encoding/json"
	"errors"
	"log"
	"net"
)

type connectionRequest struct {
	Username string `json:"username"`
	Hosting  bool   `json:"hosting"`
	RoomId   string `json:"roomId"`
}

type connectionResponse struct {
	Success      bool   `json:"success"`
	ErrorMessage string `json:"errorMessage"`
}

type playerConn struct {
	username string
	addr     net.Addr
}

type Server struct {
	room        string
	sessions    [2]*playerConn
	listener    *net.UDPConn
	playerCount int
}

func Init(port int) Server {
	server := Server{}

	addr := net.UDPAddr{
		IP:   net.IPv4(0, 0, 0, 0),
		Port: port,
		Zone: "",
	}

	listener, err := net.ListenUDP("udp", &addr)
	if err != nil {
		log.Fatal(err)
	}

	server.room = string("")
	server.listener = listener
	server.playerCount = 0

	return server
}

func (server *Server) Serve() {
	for {
		buf := make([]byte, 1024)
		n, raddr, err := server.listener.ReadFrom(buf)
		if err != nil {
			log.Println(err)
			continue
		}
		log.Println("read", n, "bytes.")

		request := connectionRequest{}
		err = json.Unmarshal(buf[:n], &request)
		if err != nil {
			log.Println(err)
			continue
		}

		response := connectionResponse{}
		_, exists := server.findPlayer(request.Username)
		if !exists {
			response, err = server.connectPlayer(request, raddr)
			if err != nil {
				log.Println(err)
				log.Println("error in connecting player", request.Username)
			}
		} else {
			response.Success = false
			response.ErrorMessage = "user already connected"
			log.Println("user already connected")
		}

		buf, err = json.Marshal(response)
		if err != nil {
			log.Println(err)
			log.Println("couldn't marshal response data to string. \n", err)
		}

		_, err = server.listener.WriteTo(buf, raddr)
		if err != nil {
			// something about dissconnect
			log.Println("could not respond to user...")
		}
	}
}

func (server *Server) connectPlayer(request connectionRequest, raddr net.Addr) (connectionResponse, error) {
	response := connectionResponse{}

	if server.playerCount == 2 {
		response.Success = false
		response.ErrorMessage = "too may players connected"
	}

	if request.Hosting && server.room != "" {
		response.Success = false
		response.ErrorMessage = "room already created"
		return response, errors.New("room already created")
	}

	if !request.Hosting && server.room == "" {
		response.Success = false
		response.ErrorMessage = "room not hosted"
		return response, errors.New("room not hosted")
	}

	if !request.Hosting && server.room != request.RoomId {
		response.Success = false
		response.ErrorMessage = "room doesn't exist"
		return response, errors.New("room doesn't exist")
	}

	if request.Hosting {
		server.room = request.RoomId
	}

	playerConn := new(playerConn)
	playerConn.username = request.Username
	playerConn.addr = raddr

	server.sessions[server.playerCount] = playerConn
	server.playerCount++

	response.Success = true
	response.ErrorMessage = ""

	log.Println(request.Username, "connected.")
	return response, nil
}

func (server *Server) findPlayer(username string) (*playerConn, bool) {
	for _, conn := range server.sessions[:2] {
		if conn != nil && conn.username == username {
			return conn, true
		}
	}

	return nil, false
}
