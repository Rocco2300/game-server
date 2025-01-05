package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/rand/v2"
	"net"
	"time"
)

type requestType int

const (
	conn requestType = iota
	playerUpdt
	collected
	death
	start
	message
)

var requestStringToEnum = map[string]requestType{
	"connection":    conn,
	"playerUpdate":  playerUpdt,
	"coinCollected": collected,
	"playerDeath":   death,
	"gameStart":     start,
	"chatMessage":   message,
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

type playerUpdate struct {
	Username string   `json:"username"`
	Health   int      `json:"health"`
	Position position `json:"position"`
}

type zoneSpawn struct {
	Scale    position `json:"scale"`
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

type coin struct {
	Id       int      `json:"id"`
	Position position `json:"position"`
}

type coinCollected struct {
	Id       int    `json:"id"`
	Username string `json:"username"`
}

type gameOver struct {
	Username string `json:"username"`
}

type playerDeath struct {
	Username string `json:"username"`
}

type chatMessage struct {
	Username string `json:"username"`
	Message  string `json:"message"`
}

type Server struct {
	listener *net.UDPConn

	room        string
	sessions    [2]*playerConn
	playerCount int

	coins          [5]*coin
	coinId         int
	collectedCoins [2]int
	gameOver       bool
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
	server.coinId = 0
	server.gameOver = false

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
		case playerUpdt:
			server.handlePlayerUpdate(requestData.(playerUpdate))
		case collected:
			server.handleCoinCollected(requestData.(coinCollected))
		case death:
			server.handlePlayerDeath(requestData.(playerDeath))
		case start:
			if server.playerCount < 2 {
				return
			}

			server.spawnPlayers()
			server.spawnCoins()

			go server.spawnZones()
		case message:
			server.handleMessage(requestData.(chatMessage))
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
	case playerUpdt:
		var rawData string
		err := json.Unmarshal(data, &rawData)
		if err != nil {
			log.Println(err)
		}

		var req playerUpdate
		err = json.Unmarshal([]byte(rawData), &req)
		if err != nil {
			log.Println(err)
		}

		return req
	case collected:
		var rawData string
		err := json.Unmarshal(data, &rawData)
		if err != nil {
			log.Println(err)
		}

		var req coinCollected
		err = json.Unmarshal([]byte(rawData), &req)
		if err != nil {
			log.Println(err)
		}

		return req
	case death:
		var rawData string
		err := json.Unmarshal(data, &rawData)
		if err != nil {
			log.Println(err)
		}

		var req playerDeath
		err = json.Unmarshal([]byte(rawData), &req)
		if err != nil {
			log.Println(err)
		}

		return req
	case start:
		return nil
	case message:
		var rawData string
		err := json.Unmarshal(data, &rawData)
		if err != nil {
			log.Println(err)
		}

		var req chatMessage
		err = json.Unmarshal([]byte(rawData), &req)
		if err != nil {
			log.Println(err)
		}

		return req
	}

	return nil
}

func (server *Server) handleConnRequest(request connectionRequest, raddr net.Addr) {
	var err error
	var connResponse connectionResponse
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
}

func (server *Server) handlePlayerUpdate(request playerUpdate) {
	buf, err := json.Marshal(request)
	if err != nil {
		log.Println("error in building broadcast player postion update message")
	}

	var response response
	response.Type = "playerUpdate"
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

func (server *Server) handleCoinCollected(request coinCollected) {
	requestBuf, err := json.Marshal(request)
	if err != nil {
		log.Println("error in building broadcast coin collected")
	}

	log.Println(request.Username, "collected coin")

	var response response
	response.Type = "coinCollected"
	response.Data = string(requestBuf)
	responseBuf, err := json.Marshal(response)
	if err != nil {
		log.Println("error in building broadcast coin collected")
	}

	for index, playerConn := range server.sessions {
		if playerConn == nil {
			continue
		}

		server.listener.WriteTo(responseBuf, playerConn.addr)

		if playerConn.username == request.Username {
			server.collectedCoins[index]++

			if server.collectedCoins[index] == 10 {
				server.signalGameOver(playerConn.username)
			}
		}
	}

	for _, coin := range server.coins {
		if coin.Id == request.Id {
			coin.Id = server.coinId
			coin.Position.X = rand.Float32()*10 - 5
			coin.Position.Y = rand.Float32()*10 - 5

			dataBuf, err := json.Marshal(coin)
			if err != nil {
				log.Println("could not marshal new coin spawn after collect data")
				log.Println(err)
			}

			response.Type = "coin"
			response.Data = string(dataBuf)
			spawnBuf, err := json.Marshal(response)
			if err != nil {
				log.Println("could not marshal new coin spawn after collect response")
				log.Println(err)
			}

			server.coinId++
			for _, playerConn := range server.sessions {
				if playerConn == nil {
					continue
				}

				server.listener.WriteTo(spawnBuf, playerConn.addr)
			}
			break
		}
	}
}

func (server *Server) signalGameOver(username string) {
	var gameOver gameOver
	gameOver.Username = username
	gameOverJson, err := json.Marshal(gameOver)
	if err != nil {
		log.Println(err)
		return
	}

	var response response
	response.Type = "gameOver"
	response.Data = string(gameOverJson)
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

	server.gameOver = true
}

func (server *Server) handlePlayerDeath(request playerDeath) {
	playerDeathJson, err := json.Marshal(request)
	if err != nil {
		log.Println(err)
		return
	}

	var response response
	response.Type = "playerDeath"
	response.Data = string(playerDeathJson)
	responseJson, err := json.Marshal(response)
	if err != nil {
		log.Println(err)
		return
	}

	var winningPlayer string
	for _, playerConn := range server.sessions {
		if playerConn == nil {
			continue
		}

		if playerConn.username != request.Username {
			winningPlayer = playerConn.username
		}

		server.listener.WriteTo(responseJson, playerConn.addr)
	}

	server.signalGameOver(winningPlayer)
}

func (server *Server) spawnPlayers() {
	var pos float32 = -1.0
	for _, player := range server.sessions {
		var spawnCommand playerUpdate
		spawnCommand.Username = player.username
		spawnCommand.Position.X = pos
		spawnCommand.Position.Y = 0.0
		spawnCommand.Health = 100

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

func (server *Server) spawnCoins() {
	for i := 0; i < 5; i++ {
		server.coins[i] = new(coin)
		server.coins[i].Id = server.coinId
		server.coins[i].Position.X = rand.Float32()*10 - 5
		server.coins[i].Position.Y = rand.Float32()*10 - 5

		server.coinId++
	}

	for _, coin := range server.coins {
		for _, playerConn := range server.sessions {
			coinJson, err := json.Marshal(coin)
			if err != nil {
				log.Println(err)
				return
			}

			response := response{}
			response.Type = "coin"
			response.Data = string(coinJson)

			responseJson, err := json.Marshal(response)
			if err != nil {
				log.Println(err)
				return
			}

			server.listener.WriteTo(responseJson, playerConn.addr)
		}
	}
}

func (server *Server) spawnZones() {
	for {
		time.Sleep(10 * time.Second)

		if server.gameOver {
			break
		}

		var zoneSpawn zoneSpawn
		zoneSpawn.Scale.X = rand.Float32()*2 + 1
		zoneSpawn.Scale.Y = rand.Float32()*2 + 1
		zoneSpawn.Position.X = rand.Float32()*10 - 5
		zoneSpawn.Position.Y = rand.Float32()*10 - 5

		zoneSpawnJson, err := json.Marshal(zoneSpawn)
		if err != nil {
			log.Println(err)
			return
		}

		var response response
		response.Type = "zoneSpawn"
		response.Data = string(zoneSpawnJson)
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
	}
}

func (server *Server) handleMessage(request chatMessage) {
	chatMessageJson, err := json.Marshal(request)
	if err != nil {
		log.Println(err)
	}

	var response response
	response.Type = "chatMessage"
	response.Data = string(chatMessageJson)
	responseJson, err := json.Marshal(response)
	if err != nil {
		log.Println(err)
	}

	for _, playerConn := range server.sessions {
		if playerConn == nil {
			continue
		}

		server.listener.WriteTo(responseJson, playerConn.addr)
	}
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
