package main

import (
	"fmt"
	socketio "github.com/googollee/go-socket.io"
	"log"
	"math/rand"
	"net/http"
	"sort"
	"strconv"
	"time"
)

type Player struct {
	Position   int
	SessionId  string
	ReadyState int
	Hands      []int
	Outsides   []int
	River      []int
}

type RoomData struct {
	RoomId   int
	Owner    string
	Players  map[string]*Player
	AllCards []int
}

type ServerResponse struct {
	EventName string      `json:"event_name"`
	Extra     interface{} `json:"extra"`
	CheckSum  string      `json:"check_sum"`
}

type FirstDealExtra struct {
	Hands       []int
	LeftHands   int
	AcrossHands int
	RightHands  int
}

var sessionIdRoomIdMap = make(map[string]int)
var sessionIdConnMap = make(map[string]socketio.Conn)

var roomIdData = make(map[int]*RoomData)

var idStart = 1000

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
		var roomId, err = strconv.Atoi(msg)
		if err != nil {
			//todo
			return
		}

		var _, ok = roomIdData[roomId]
		if !ok {
			//todo
			return
		}

		var sessionId = s.RemoteHeader().Get("sessionId")
		sessionIdRoomIdMap[sessionId] = roomId
		var player = &Player{Hands: make([]int, 13), SessionId: sessionId}
		s.Join(msg)
		roomIdData[roomId].Players[sessionId] = player
		fmt.Println(sessionId + " join " + msg)
		s.Emit("join", msg)
	})

	server.OnEvent("/", "create", func(s socketio.Conn, msg string) {
		var sessionId = s.RemoteHeader().Get("sessionId")
		var roomId = generateNewRoomId()
		var roomStr = strconv.Itoa(roomId)
		var player = &Player{Hands: make([]int, 13), SessionId: sessionId}
		s.Join(roomStr)

		var _, ok = roomIdData[roomId]
		if !ok {
			roomIdData[roomId] = &RoomData{RoomId: roomId, Players: make(map[string]*Player), Owner: sessionId}
		}
		roomIdData[roomId].Players[sessionId] = player
		sessionIdRoomIdMap[sessionId] = roomId
		fmt.Println(sessionId + " create " + roomStr)
		s.Emit("create", roomStr)
	})

	server.OnEvent("/", "ready", func(s socketio.Conn, msg string) {
		var sessionId = s.RemoteHeader().Get("sessionId")
		var roomId, ok = sessionIdRoomIdMap[sessionId]
		if ok {
			roomIdData[roomId].Players[sessionId].ReadyState = 1
			s.Emit("ready", "")
			var isAllReady = isAllReady(roomId)
			if isAllReady {
				var owner = roomIdData[roomId].Owner
				sessionIdConnMap[owner].Emit("allReady", "")
			}
		}
		fmt.Println(sessionId + " ready ")
	})
	server.OnEvent("/", "unready", func(s socketio.Conn, msg string) {
		var sessionId = s.RemoteHeader().Get("sessionId")
		var roomId, ok = sessionIdRoomIdMap[sessionId]
		if ok {
			roomIdData[roomId].Players[sessionId].ReadyState = 0
			s.Emit("unready", "")
			var owner = roomIdData[roomId].Owner
			sessionIdConnMap[owner].Emit("notAllReady", "")
		}
		fmt.Println(sessionId + " unready ")
	})

	server.OnEvent("/", "startGame", func(s socketio.Conn, msg string) {
		var sessionId = s.RemoteHeader().Get("sessionId")
		var roomId, ok = sessionIdRoomIdMap[sessionId]
		if ok {
			startGame(server, roomId)
		}
		fmt.Println(sessionId + " start game ")
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
	log.Println("Serving at localhost:7000...")
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

func startGame(server *socketio.Server, roomId int) {
	var roomData = roomIdData[roomId]
	initRoomData(roomData)
	firstDeal(roomData)
	server.BroadcastToRoom("/", strconv.Itoa(roomId), "startGame")
}

func actionFirstHand(s socketio.Conn, sessionId string, roomId int) {
	var roomData = roomIdData[roomId]
	var players = make([]*Player, 0, 4)
	for i := 0; i < 4; i++ {
		players = append(players, getPlayerForPosition(roomData, i))
	}
	for i := 0; i < 4; i++ {
		var player = players[i]
		if player.SessionId != sessionId {
			continue
		}
		var left = (i + 4 - 1) % 4
		var across = (i + 4 - 2) % 4
		var right = (i + 4 - 3) % 4
		var extra = &FirstDealExtra{Hands: player.Hands, LeftHands: len(players[left].Hands), AcrossHands: len(players[across].Hands), RightHands: len(players[right].Hands)}
		var res = &ServerResponse{EventName: "actionFirstHand", Extra: extra}
		serverRespond(s, res, player.Position)
	}
}

func isAllReady(roomId int) bool {
	var allReady bool = true
	var roomCap = 4
	var roomData = roomIdData[roomId]
	var roomPlayerCount = len(roomData.Players)
	if roomCap == roomPlayerCount {
		for _, player := range roomData.Players {
			if player.ReadyState == 0 {
				allReady = false
			}
		}
	} else {
		allReady = false
	}
	return allReady
}

func initRoomData(data *RoomData) {
	initAllCards(data)
	initPlayerPosition(data)
}

func initAllCards(data *RoomData) {
	var allCards = make([]int, 0, 108)
	for i := 0; i < 4; i++ {
		for i := WAN_1; i <= WAN_9; i++ {
			allCards = append(allCards, i)
		}
		for i := PIN_1; i <= PIN_9; i++ {
			allCards = append(allCards, i)
		}
		for i := SUO_1; i <= SUO_9; i++ {
			allCards = append(allCards, i)
		}
	}
	rand.Seed(time.Now().UnixNano())
	rand.Shuffle(len(allCards), func(i, j int) { allCards[i], allCards[j] = allCards[j], allCards[i] })
}

func initPlayerPosition(data *RoomData) {
	var i = 0
	for _, player := range data.Players {
		player.Position = i
		i++
	}
}

func firstDeal(data *RoomData) {
	for _, player := range data.Players {
		var hands = data.AllCards[0:13]
		sortHands(hands)
		player.Hands = hands
		data.AllCards = data.AllCards[13:]
	}
	var firstPlayer = getPlayerForPosition(data, 0)
	var card = data.AllCards[0]
	firstPlayer.Hands = append(firstPlayer.Hands, card)
	sortHands(firstPlayer.Hands)
	data.AllCards = data.AllCards[1:]
}

func getPlayerForPosition(data *RoomData, pos int) *Player {
	for _, player := range data.Players {
		if player.Position == pos {
			return player
		}
	}
	return nil
}

func sortHands(hands []int) {
	sort.Ints(hands)
}

func addCheckSum(res *ServerResponse, position int) {
	res.CheckSum = ""
}

func serverRespond(s socketio.Conn, res *ServerResponse, position int) {
	addCheckSum(res, position)
	s.Emit(res.EventName, res)
}

func generateNewRoomId() int {
	var res = idStart
	idStart++
	return res
}
