package main

import "game-server/server"

func main() {
	s := server.Init(12345)

	s.Serve()
}
