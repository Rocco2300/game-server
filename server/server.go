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
	Username     string `json:"username"`
	Success      bool   `json:"success"`
	ErrorMessage string `json:"errorMessage"`
}

type position struct {
	X float32 `json:"x"`
	Y float32 `json:"y"`
}

type spawnCommand struct {
	Username string   `json:"username"`
	Position position `json:"position"`
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
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

type response struct {
	Type string `json:"type"`
	Data string `json:"data"`
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

		reqType, err := server.getRequestType(buf, n)
		if err != nil {
			log.Println(err)
			continue
		}

		request := request{}
		err = json.Unmarshal(buf[:n], &request)
		if err != nil {
			log.Println("error in parsing request")
			log.Println(err)
			continue
		}

		requestData := server.buildRequest(request.Data, reqType)
		switch reqType {
		case conn:
			server.handleConnRequest(requestData.(connectionRequest), raddr)
		case posUpdate:
			server.handlePosUpdate(requestData.(playerPositionUpdate))
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

	reqTypeStr := requestStringToEnum[outer.Type]
	return reqTypeStr, nil
}

func (server *Server) buildRequest(data json.RawMessage, requestType requestType) any {
	switch requestType {
	case conn:
		var rawData string
		err := json.Unmarshal(data, &rawData)
		if err != nil {
			log.Println(err)
		}

		var req connectionRequest
		err = json.Unmarshal([]byte(rawData), &req)
		if err != nil {
			log.Println(err)
		}

		return req
	case posUpdate:
		var rawData string
		err := json.Unmarshal(data, &rawData)
		if err != nil {
			log.Println(err)
		}

		var req playerPositionUpdate
		err = json.Unmarshal([]byte(rawData), &req)
		if err != nil {
			log.Println(err)
		}

		return req
	}

	return nil
}

func (server *Server) handleConnRequest(request connectionRequest, raddr net.Addr) {
	err := errors.New("")
	connResponse := connectionResponse{}
	_, exists := server.findPlayer(request.Username)
	if !exists {
		connResponse, err = server.connectPlayer(request, raddr)
		if err != nil {
			errMsg := fmt.Sprintf("error in connecting %s", request.Username)

			log.Println(err)
			log.Println(errMsg)
		}
	} else {
		errMsg := fmt.Sprintf("user %s already connected", request.Username)
		log.Println(errMsg)
	}

	connResponseJSON, err := json.Marshal(connResponse)
	if err != nil {
		log.Println(err)
		log.Println("could not marshal response data string")
	}

	response := response{}
	response.Type = "connection"
	response.Data = string(connResponseJSON)
	buf, err := json.Marshal(response)
	if err != nil {
		log.Println(err)
		log.Println("could not marshal response")
	}

	_, err = server.listener.WriteTo(buf, raddr)
	if err != nil {
		errMsg := fmt.Sprintf("could not respond to user %s", request.Username)
		// something about disconnect
		log.Println(err)
		log.Println(errMsg)
	}

	if !connResponse.Success {
		return
	}

	if server.playerCount < 2 {
		return
	}

	server.spawnPlayers()
	server.spawnCoins()
}

func (server *Server) handlePosUpdate(request playerPositionUpdate) {
	buf, err := json.Marshal(request)
	if err != nil {
		log.Println("error in building broadcast player postion update message")
	}

	var response response
	response.Type = "positionUpdate"
	response.Data = string(buf)
	buf, err = json.Marshal(response)
	if err != nil {
		log.Println("error in building broadcast player position update message")
	}

	for _, playerConn := range server.sessions {
		if playerConn == nil {
			continue
		}

		server.listener.WriteTo(buf, playerConn.addr)
	}
}

func (server *Server) spawnPlayers() {
	var pos float32 = -1.0
	for _, player := range server.sessions {
		var spawnCommand spawnCommand
		spawnCommand.Username = player.username
		spawnCommand.Position.X = pos
		spawnCommand.Position.Y = 0.0

		spawnCommandJson, err := json.Marshal(spawnCommand)
		if err != nil {
			log.Println(err)
			return
		}

		var response response
		response.Type = "spawn"
		response.Data = string(spawnCommandJson)

		responseJson, err := json.Marshal(response)
		if err != nil {
			log.Println(err)
			return
		}

		for _, playerConn := range server.sessions {
			if playerConn == nil {
				continue
			}

			server.listener.WriteTo(responseJson, playerConn.addr)
		}

		pos += 2
	}
}

func (serve *Server) spawnCoins() {

}

func (server *Server) connectPlayer(request connectionRequest, raddr net.Addr) (connectionResponse, error) {
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

	response.Username = request.Username
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
