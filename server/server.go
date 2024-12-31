package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
)

type requestType int

const (
	conn requestType = iota
	posUpdate
)

var requestStringToEnum = map[string]requestType{
	"connection":     conn,
	"positionUpdate": posUpdate,
}

type connectionRequest struct {
	Username string `json:"username"`
	Hosting  bool   `json:"hosting"`
	RoomId   string `json:"roomId"`
}

type connectionResponse struct {
	Success      bool   `json:"success"`
	ErrorMessage string `json:"errorMessage"`
}

type position struct {
	X float32 `json:"x"`
	Y float32 `json:"y"`
}

type playerPositionUpdate struct {
	Username string   `json:"username"`
	Position position `json:"position"`
}

type playerConn struct {
	username string
	addr     net.Addr
}

type request struct {
	Type string `json:"type"`
	Data any    `json:"data"`
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

		reqType, err := server.getRequestType(buf, n)
		if err != nil {
			log.Println(err)
			continue
		}

		request := request{}
		server.buildRequest(&request, reqType)
		err = json.Unmarshal(buf[:n], &request)
		if err != nil {
			log.Println("error in parsing request")
			continue
		}

		switch reqType {
		case conn:
			err := server.handleConnRequest(request.Data.(*connectionRequest), raddr)
			if err != nil {
				log.Println(err)
			}
		case posUpdate:
			err := server.handlePosUpdate(request.Data.(*playerPositionUpdate))
			if err != nil {
				log.Println(err)
			}
		}
	}
}

func (server *Server) getRequestType(buf []byte, n int) (requestType, error) {
	var outer struct {
		Type string `json:"type"`
	}

	err := json.Unmarshal(buf[:n], &outer)
	if err != nil {
		log.Println("could not get request type")
		log.Println(err)

		return conn, errors.New("could not get request type")
	}
	fmt.Printf("request type %s\n", outer.Type)

	reqTypeStr := requestStringToEnum[outer.Type]
	return reqTypeStr, nil
}

func (server *Server) buildRequest(request *request, requestType requestType) {
	switch requestType {
	case conn:
		request.Data = new(connectionRequest)
	case posUpdate:
		request.Data = new(playerPositionUpdate)
	}
}

func (server *Server) handleConnRequest(request *connectionRequest, raddr net.Addr) error {
	err := errors.New("")
	response := connectionResponse{}
	_, exists := server.findPlayer(request.Username)
	if !exists {
		response, err = server.connectPlayer(request, raddr)
		if err != nil {
			errMsg := fmt.Sprintf("error in connecting %s", request.Username)

			log.Println(err)
			return errors.New(errMsg)
		}
	} else {
		errMsg := fmt.Sprintf("user %s already connected", request.Username)

		response.Success = false
		response.ErrorMessage = errMsg
		return errors.New(errMsg)
	}

	buf, err := json.Marshal(response)
	if err != nil {
		log.Println(err)
		return errors.New("couldn't marshal response data to string")
	}

	_, err = server.listener.WriteTo(buf, raddr)
	if err != nil {
		errMsg := fmt.Sprintf("could not respond to user %s", request.Username)
		// something about disconnect
		log.Println(err)
		return errors.New(errMsg)
	}

	return nil
}

func (server *Server) handlePosUpdate(request *playerPositionUpdate) error {
	buf, err := json.Marshal(request)
	if err != nil {
		return errors.New("error in building broadcast player postion update message")
	}

	for _, playerConn := range server.sessions {

		server.listener.WriteTo(buf, playerConn.addr)
	}

	return nil
}

func (server *Server) connectPlayer(request *connectionRequest, raddr net.Addr) (connectionResponse, error) {
	response := connectionResponse{}

	if server.playerCount == 2 {
		response.Success = false
		response.ErrorMessage = "too many players connected"
		return response, errors.New("too many players connected")
	}

	if request.Hosting && server.room != "" {
		errMsg := fmt.Sprintf("room %s already created", request.RoomId)

		response.Success = false
		response.ErrorMessage = errMsg
		return response, errors.New(errMsg)
	}

	if !request.Hosting && server.room == "" {
		errMsg := fmt.Sprintf("room %s already created", request.RoomId)

		response.Success = false
		response.ErrorMessage = errMsg
		return response, errors.New(errMsg)
	}

	if !request.Hosting && server.room != request.RoomId {
		errMsg := fmt.Sprintf("room %s doesn't exist", request.RoomId)

		response.Success = false
		response.ErrorMessage = errMsg
		return response, errors.New(errMsg)
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
