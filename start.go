package main

import (
	"fmt"
	socketio "github.com/googollee/go-socket.io"
	"log"
	"net/http"
)

type Player struct {
	ReadyState int
	Cards      []int
}

var sessionIdRoomIdMap = make(map[string]string)
var sessionIdConnMap = make(map[string]socketio.Conn)
var roomIdSessionIdMap = make(map[string]map[string]*Player)

func main() {
	server := socketio.NewServer(nil)

	server.OnConnect("/", func(s socketio.Conn) error {
		s.SetContext("")
		var sessionId = s.RemoteHeader().Get("sessionId")
		fmt.Println("connected:", s.ID())
		fmt.Println("connected:", sessionId)
		var v, ok = sessionIdConnMap[sessionId]
		if ok {
			v.LeaveAll()
			v.Close()
		}
		sessionIdConnMap[sessionId] = s
		return nil
	})

	server.OnEvent("/", "join", func(s socketio.Conn, msg string) {
		var sessionId = s.RemoteHeader().Get("sessionId")
		sessionIdRoomIdMap[sessionId] = msg
		var _, ok = roomIdSessionIdMap[msg]
		if !ok {
			roomIdSessionIdMap[msg] = make(map[string]*Player)
		}
		roomIdSessionIdMap[msg][sessionId] = &Player{Cards: make([]int, 13)}
		s.Join(msg)
		fmt.Println(sessionId + " join " + msg)
		s.Emit("join", "")
	})

	server.OnEvent("/", "ready", func(s socketio.Conn, msg string) {
		var sessionId = s.RemoteHeader().Get("sessionId")
		var roomId, ok = sessionIdRoomIdMap[sessionId]
		if ok {
			roomIdSessionIdMap[roomId][sessionId].ReadyState = 1
			dealReady(server, roomId)
		}
		fmt.Println(sessionId + " ready ")
	})
	server.OnEvent("/", "unready", func(s socketio.Conn, msg string) {
		var sessionId = s.RemoteHeader().Get("sessionId")
		var roomId, ok = sessionIdRoomIdMap[sessionId]
		if ok {
			roomIdSessionIdMap[roomId][sessionId].ReadyState = 0
			dealReady(server, roomId)
		}
		fmt.Println(sessionId + " ready ")
	})

	server.OnError("/", func(s socketio.Conn, e error) {
		fmt.Println("meet error:", e)
	})

	server.OnDisconnect("/", func(s socketio.Conn, reason string) {
		fmt.Println("closed", reason)
	})

	go server.Serve()
	defer server.Close()

	http.HandleFunc("/socket.io/", func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Access-Control-Allow-Origin", request.Header.Get("origin"))
		writer.Header().Set("Access-Control-Allow-Methods", "GET, POST")
		writer.Header().Set("Access-Control-Allow-Credentials", "true")
		request.Header.Del("Origin")
		server.ServeHTTP(writer, request)
	})
	http.Handle("/", http.FileServer(http.Dir("./asset")))
	log.Println("Serving at localhost:8000...")
	log.Fatal(http.ListenAndServe("127.0.0.1:7000", nil))
	//var upgrader = websocket.Upgrader{}
	//http.HandleFunc("/ws", func(writer http.ResponseWriter, request *http.Request) {
	//	var conn, _ = upgrader.Upgrade(writer, request, nil)
	//	go func(conn *websocket.Conn) {
	//		for {
	//			var mType, msg, _ = conn.ReadMessage()
	//			fmt.Println(msg)
	//			conn.WriteMessage(mType, msg)
	//		}
	//	}(conn)
	//})
	//http.ListenAndServe("127.0.0.1:8100", nil)
}

func dealReady(server *socketio.Server, roomId string) {
	var allReady bool = true
	var roomCap = 1
	var roomPlayerCount = len(roomIdSessionIdMap[roomId])
	if roomCap == roomPlayerCount {
		for _, player := range roomIdSessionIdMap[roomId] {
			if player.ReadyState == 0 {
				allReady = false
			}
		}
	} else {
		allReady = false
	}
	if allReady {
		server.BroadcastToRoom("/", roomId, "gameStart")
	}
}
