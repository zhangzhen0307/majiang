package main

import (
	"bytes"
	"context"
	"encoding/gob"
	"encoding/json"
	"fmt"
	socketio "github.com/googollee/go-socket.io"
	"log"
	"math/rand"
	"net/http"
	"sort"
	"strconv"
	"time"
)

const (
	ActionIgnore = iota
	ActionPeng
	ActionGang1 //内晃
	ActionGang2 //外晃
	ActionGang3 //点晃
	ActionHu
	ActionZimo
	ActionLaizi
)

type User struct {
	Id   int
	Name string
}

type Player struct {
	Position   int
	UserId     int
	Name       string
	ReadyState int
	Hands      []Card
	Outsides   map[int][]Card
	River      []Card
	NewCard    Card
}

type RoomData struct {
	RoomId                int
	Owner                 int
	Players               map[int]*Player
	PlayersByPosition     map[int]*Player
	AllCards              []Card
	Laizi                 int
	AllCardsMap           map[int]int
	CurrentActiveCard     Card
	CurrentActivePLayer   int
	CurrentState          int //0-出牌前等待action,1-出牌后等待action
	WaitActionPlayerCount int // currentstate = 1时需要等待多少人的action
	DoneActions           map[int]int
	Winner                int
	IgnoreTimer           []*time.Timer
	ChupaiTimer           *time.Timer
}

type Card struct {
	Num int
	Uid int
}

type ServerResponse struct {
	EventName   string      `json:"event_name"`
	Extra       interface{} `json:"extra"`
	Actions     []int       `json:"actions"`
	ActionExtra interface{} `json:"action_extra"`
	CheckSum    string      `json:"check_sum"`
}

type OtherJoinExtra struct {
	Id   int    `json:"id"`
	Name string `json:"name"`
}

type RoomInfoPlayer struct {
	Id         int    `json:"id"`
	Name       string `json:"name"`
	ReadyState int    `json:"ready_state"`
	IsSelf     int    `json:"is_self"`
}

type RoomInfoExtra struct {
	Players []RoomInfoPlayer `json:"players"`
}

type AllDataExtra struct {
	Hands       []Card
	LeftHands   int
	AcrossHands int
	RightHands  int
	MyPlayerPos int
	Laizi       int
}

type readyExtra struct {
	ReadyState int `json:"ready_state"`
	Id         int `json:"id"`
}

type ChupaiExtra struct {
	Position int
	OutCard  Card
}

type DealCardExtra struct {
	DealCard Card
	Position int
}

type PengExtra struct {
	PengCards []Card
	Position  int
}

type Gang1Extra struct {
	GangCards []Card
	Position  int
}

type Gang2Extra struct {
	GangCards []Card
	Position  int
}

type Gang3Extra struct {
	GangCards []Card
	Position  int
}

type LaiziExtra struct {
	Card     Card
	Position int
}

type ZimoExtra struct {
	Cards    []Card
	Position int
}

type GameOverExtra struct {
	Winner string
}

var UserIdRoomIdMap = make(map[int]int)
var UserIdConnMap = make(map[int]socketio.Conn)

var roomIdData = make(map[int]*RoomData)

var idStart = 1000

func autoIgnore(uuids []int) []*time.Timer {
	var timers = make([]*time.Timer, len(uuids))
	for i, uuid := range uuids {
		timers[i] = time.AfterFunc(10*time.Second, func() {
			actionIgnore(uuid)
		})
	}
	return timers
}

func autoChupai(uuid int, outCardUid int) *time.Timer {
	return time.AfterFunc(10*time.Second, func() {
		actionChupai(uuid, outCardUid)
	})
}

func main() {
	//var successcount = 0
	//var rands = make([][]Card, 0)
	//var lcount = 1
	//for i := 0; i < 100000; i++ {
	//	var cards, _ = randCards()
	//	rands = append(rands, cards)
	//}
	//var time1 = time.Now().UnixMilli()
	//for _, r := range rands {
	//	var r = isHu(r, 0, lcount)
	//	if r {
	//		successcount += 1
	//	}
	//}
	//var time2 = time.Now().UnixMilli()
	//var diff = time2 - time1
	//fmt.Println(diff)
	//fmt.Println(successcount)
	//return
	//var cards = make([]Card, 0)
	//cards = append(cards, Card{Num: 1})
	//cards = append(cards, Card{Num: 1})
	//cards = append(cards, Card{Num: 1})
	//cards = append(cards, Card{Num: 2})
	//cards = append(cards, Card{Num: 3})
	//cards = append(cards, Card{Num: 4})
	//cards = append(cards, Card{Num: 5})
	//cards = append(cards, Card{Num: 6})
	//cards = append(cards, Card{Num: 7})
	//cards = append(cards, Card{Num: 8})
	//cards = append(cards, Card{Num: 9})
	//cards = append(cards, Card{Num: 9})
	////cards = append(cards, Card{Num: 101})
	//cards = append(cards, Card{Num: 5})
	//var r = isHu(cards, 0, 1)
	//fmt.Println(r)
	//return
	//var seid = "session_AMKYZE6YWRQ7TBYQNFJR4CPG3VJTZ6ETEFF2NA3K44ZYN3RJPPLQ"
	//var rdb = GetRedis()
	//var ctx = context.Background()
	//
	//val, err := rdb.Get(ctx, seid).Result()
	//if err != nil {
	//	panic(err)
	//}

	//var result = map[interface{}]interface{}{}
	//dec := gob.NewDecoder(bytes.NewBuffer([]byte(val)))
	//err = dec.Decode(&result)
	//if err != nil {
	//	fmt.Println(err)
	//}
	//fmt.Println("key", val)
	//fmt.Println("---------")
	//fmt.Println(result)
	//fmt.Println("-------")
	//fmt.Println(result["uuid"])
	//return
	server := socketio.NewServer(nil)

	server.OnConnect("/", func(s socketio.Conn) error {
		s.SetContext("")
		var sessionId = s.RemoteHeader().Get("sessionId")
		var user = getUserFromSessionId(sessionId)
		fmt.Println("connected:", s.ID())
		fmt.Println("connected:", user.Name)
		var v, ok = UserIdConnMap[user.Id]
		if ok {
			v.LeaveAll()
			//v.Close()
		}
		UserIdConnMap[user.Id] = s
		//s.Emit("onConnect", "")
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
		var user = getUserFromSessionId(sessionId)
		UserIdRoomIdMap[user.Id] = roomId
		var player = &Player{Hands: make([]Card, 0), UserId: user.Id, Name: user.Name, Outsides: make(map[int][]Card), River: make([]Card, 0)}
		s.Join(msg)
		roomIdData[roomId].Players[user.Id] = player
		fmt.Println(user.Name + " join " + msg)
		s.Emit("join", msg)
		var extra = OtherJoinExtra{Id: user.Id, Name: user.Name}
		var res = ServerResponse{EventName: "otherJoin", Extra: extra}
		for _, player := range roomIdData[roomId].Players {
			if player.UserId != user.Id {
				serverRespond(UserIdConnMap[player.UserId], res, player.Position)
			}
		}
	})

	server.OnEvent("/", "create", func(s socketio.Conn, msg string) {
		var sessionId = s.RemoteHeader().Get("sessionId")
		var user = getUserFromSessionId(sessionId)
		var roomId = generateNewRoomId()
		var roomStr = strconv.Itoa(roomId)
		var player = &Player{Hands: make([]Card, 13), UserId: user.Id, Name: user.Name, Outsides: make(map[int][]Card), River: make([]Card, 0)}
		s.Join(roomStr)

		var _, ok = roomIdData[roomId]
		if !ok {
			roomIdData[roomId] = &RoomData{RoomId: roomId, Players: make(map[int]*Player), Owner: user.Id, CurrentActivePLayer: -1, Winner: -1}
		}
		roomIdData[roomId].Players[user.Id] = player
		UserIdRoomIdMap[user.Id] = roomId
		fmt.Println(user.Name + " create " + roomStr)
		s.Emit("create", roomStr)
		//var all = &AllDataExtra{Hands: nil, LeftHands: 1,RightHands: 1,AcrossHands: 2}
		//var d = &ServerResponse{EventName: "aaa", Extra: all}
		//s.Emit("create", d)
	})

	server.OnEvent("/", "getRoomInfo", func(s socketio.Conn, msg string) {
		var sessionId = s.RemoteHeader().Get("sessionId")
		var user = getUserFromSessionId(sessionId)
		var roomId, ok = UserIdRoomIdMap[user.Id]
		if ok {
			var roomData = roomIdData[roomId]
			var extra = RoomInfoExtra{Players: make([]RoomInfoPlayer, 0)}
			for _, player := range roomData.Players {
				var name = player.Name
				var readyState = player.ReadyState
				if user.Id == player.UserId {
					extra.Players = append(extra.Players, RoomInfoPlayer{Id: player.UserId, Name: name, ReadyState: readyState, IsSelf: 1})
				} else {
					extra.Players = append(extra.Players, RoomInfoPlayer{Id: player.UserId, Name: name, ReadyState: readyState, IsSelf: 0})
				}
			}
			var res = ServerResponse{EventName: "getRoomInfo", Extra: extra}
			serverRespond(s, res, 0)
		}
		fmt.Println(user.Name + " getroominfo ")
	})

	server.OnEvent("/", "ready", func(s socketio.Conn, msg string) {
		var sessionId = s.RemoteHeader().Get("sessionId")
		var user = getUserFromSessionId(sessionId)
		var roomId, ok = UserIdRoomIdMap[user.Id]
		if ok {
			roomIdData[roomId].Players[user.Id].ReadyState = 1
			var extra = readyExtra{ReadyState: 1, Id: user.Id}
			var res = ServerResponse{EventName: "ready", Extra: extra}
			for _, player := range roomIdData[roomId].Players {
				serverRespond(UserIdConnMap[player.UserId], res, player.Position)
			}
			var isAllReady = isAllReady(roomId)
			if isAllReady {
				var owner = roomIdData[roomId].Owner
				UserIdConnMap[owner].Emit("allReady", "")
			}
		}
		fmt.Println(user.Name + " ready ")
	})
	server.OnEvent("/", "unready", func(s socketio.Conn, msg string) {
		var sessionId = s.RemoteHeader().Get("sessionId")
		var user = getUserFromSessionId(sessionId)
		var roomId, ok = UserIdRoomIdMap[user.Id]
		if ok {
			roomIdData[roomId].Players[user.Id].ReadyState = 0
			var extra = readyExtra{ReadyState: 0, Id: user.Id}
			var res = ServerResponse{EventName: "ready", Extra: extra}
			for _, player := range roomIdData[roomId].Players {
				serverRespond(UserIdConnMap[player.UserId], res, player.Position)
			}
			var owner = roomIdData[roomId].Owner
			UserIdConnMap[owner].Emit("notAllReady", "")
		}
		fmt.Println(user.Name + " unready ")
	})

	server.OnEvent("/", "startGame", func(s socketio.Conn, msg string) {
		fmt.Println("startgame")
		var sessionId = s.RemoteHeader().Get("sessionId")
		var user = getUserFromSessionId(sessionId)
		var roomId, ok = UserIdRoomIdMap[user.Id]
		if ok {
			startGame(server, roomId)
		}
		fmt.Println(user.Name + " start game ")
	})

	server.OnEvent("/", "actionAllData", func(s socketio.Conn, msg string) {
		var sessionId = s.RemoteHeader().Get("sessionId")
		var user = getUserFromSessionId(sessionId)
		actionAllData(s, user.Id)
	})

	server.OnEvent("/", "actionChupai", func(s socketio.Conn, msg string) {
		var sessionId = s.RemoteHeader().Get("sessionId")
		var user = getUserFromSessionId(sessionId)
		fmt.Println("action chupai on " + user.Name + " chu: " + msg)
		var outCard, err = strconv.Atoi(msg)
		if err != nil {
			//todo
			return
		}
		actionChupai(user.Id, outCard)
	})

	server.OnEvent("/", "actionIgnore", func(s socketio.Conn, msg string) {
		var sessionId = s.RemoteHeader().Get("sessionId")
		var user = getUserFromSessionId(sessionId)
		fmt.Println("action ignore on " + user.Name)
		actionIgnore(user.Id)
	})

	server.OnEvent("/", "actionPeng", func(s socketio.Conn, msg string) {
		var sessionId = s.RemoteHeader().Get("sessionId")
		var user = getUserFromSessionId(sessionId)
		fmt.Println("action peng on " + user.Name)
		actionPeng(s, user.Id)
	})

	server.OnEvent("/", "actionGang1", func(s socketio.Conn, msg string) {
		var sessionId = s.RemoteHeader().Get("sessionId")
		var user = getUserFromSessionId(sessionId)
		fmt.Println("action gang1 on " + user.Name)
		var num, err = strconv.Atoi(msg)
		if err != nil {
			//todo
			return
		}
		actionGang1(s, user.Id, num)
	})

	server.OnEvent("/", "actionGang2", func(s socketio.Conn, msg string) {
		var sessionId = s.RemoteHeader().Get("sessionId")
		var user = getUserFromSessionId(sessionId)
		fmt.Println("action gang2 on " + user.Name)
		actionGang2(s, user.Id)
	})

	server.OnEvent("/", "actionGang3", func(s socketio.Conn, msg string) {
		var sessionId = s.RemoteHeader().Get("sessionId")
		var user = getUserFromSessionId(sessionId)
		fmt.Println("action gang3 on " + user.Name)
		actionGang3(s, user.Id)
	})

	server.OnEvent("/", "actionLaizi", func(s socketio.Conn, msg string) {
		var sessionId = s.RemoteHeader().Get("sessionId")
		var user = getUserFromSessionId(sessionId)
		fmt.Println("action laizi on " + user.Name)
		actionLaizi(s, user.Id)
	})

	server.OnEvent("/", "actionZimo", func(s socketio.Conn, msg string) {
		var sessionId = s.RemoteHeader().Get("sessionId")
		var user = getUserFromSessionId(sessionId)
		fmt.Println("action zimo on " + user.Name)
		actionZimo(s, user.Id)
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
	if Config.Mode == RUNMODE_LOCAL {
		log.Fatal(http.ListenAndServe(Config.ServerRunAddress, nil))
	} else {
		log.Fatal(http.ListenAndServeTLS(Config.ServerRunAddress, Config.TlsPemFile, Config.TlsKeyFile, nil))
	}
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

func clearIgnoreTimer(roomData *RoomData) {
	if roomData.IgnoreTimer != nil {
		for _, timer := range roomData.IgnoreTimer {
			timer.Stop()
		}
		roomData.IgnoreTimer = nil
	}
}

func clearChupaiTimer(roomData *RoomData) {
	if roomData.ChupaiTimer != nil {
		roomData.ChupaiTimer.Stop()
		roomData.ChupaiTimer = nil
	}
}

func startGame(server *socketio.Server, roomId int) {
	var roomData = roomIdData[roomId]
	initRoomData(roomData)
	firstDeal(roomData)
	roomData.WaitActionPlayerCount = 4
	server.BroadcastToRoom("/", strconv.Itoa(roomId), "startGame", "")
}

func firstDeal(data *RoomData) {
	for _, player := range data.Players {
		var hands = make([]Card, 13)
		copy(hands, data.AllCards[0:13])
		sortHands(hands)
		player.Hands = hands
		data.AllCards = data.AllCards[13:]
	}
}

func actionAllData(s socketio.Conn, uuid int) {
	var roomId = UserIdRoomIdMap[uuid]
	var roomData = roomIdData[roomId]
	var players = roomData.PlayersByPosition
	for i := 0; i < 4; i++ {
		var player = players[i]
		if player.UserId != uuid {
			continue
		}
		var left = (i + 4 - 1) % 4
		var across = (i + 4 - 2) % 4
		var right = (i + 4 - 3) % 4
		var extra = AllDataExtra{Hands: player.Hands, LeftHands: len(players[left].Hands), AcrossHands: len(players[across].Hands), RightHands: len(players[right].Hands), MyPlayerPos: player.Position, Laizi: roomData.Laizi}
		var res = ServerResponse{EventName: "actionAllData", Actions: make([]int, 0), Extra: extra}
		serverRespond(s, res, player.Position)
		roomData.WaitActionPlayerCount -= 1
	}
	if roomData.WaitActionPlayerCount == 0 {
		dealNextCard(roomData, -1)
	}
}

func actionChupai(uuid int, outCardUid int) {
	defer func() {
		if e := recover(); e != nil {
			fmt.Println("actionchupai error ")
			fmt.Println(e)
			panic(e)
		}
	}()
	var roomId = UserIdRoomIdMap[uuid]
	var roomData = roomIdData[roomId]
	var players = roomData.PlayersByPosition
	var activePlayerIndex = -1
	for _, player := range players {
		if player.UserId == uuid {
			activePlayerIndex = player.Position
			continue
		}
	}

	var actionUuids = make([]int, 0)
	for i := 0; i < 4; i++ {
		if i == activePlayerIndex {
			var self = players[i]
			if self.NewCard.Num > 0 {
				self.Hands = append(self.Hands, self.NewCard)
			}
			var index = -1
			for in, v := range self.Hands {
				if v.Uid == outCardUid {
					index = in
					break
				}
			}
			var c = self.Hands[index]
			self.Hands = append(self.Hands[:index], self.Hands[(index+1):]...)
			sortHands(self.Hands)
			self.NewCard = Card{}
			self.River = append(self.River, c)
			var extra = ChupaiExtra{Position: activePlayerIndex, OutCard: c}
			var res = ServerResponse{EventName: "actionChupai", Extra: extra, Actions: make([]int, 0)}
			serverRespond(UserIdConnMap[self.UserId], res, self.Position)
		} else {
			var player = players[i]
			var conn = UserIdConnMap[player.UserId]
			var c = Card{Num: roomData.AllCardsMap[outCardUid], Uid: outCardUid}
			var extra = ChupaiExtra{Position: activePlayerIndex, OutCard: c}
			var res = ServerResponse{EventName: "actionChupai", Extra: extra}
			res.Actions = getActionsAfterChupai(player, Card{Num: roomData.AllCardsMap[outCardUid], Uid: outCardUid})
			serverRespond(conn, res, player.Position)
			if len(res.Actions) > 0 {
				roomData.WaitActionPlayerCount += 1
				roomData.DoneActions = make(map[int]int)
				roomData.CurrentState = 1
				actionUuids = append(actionUuids, player.UserId)
			}
		}
	}
	roomData.CurrentActiveCard = Card{Num: roomData.AllCardsMap[outCardUid], Uid: outCardUid}

	clearChupaiTimer(roomData)

	if roomData.WaitActionPlayerCount == 0 {
		dealNextCard(roomData, -1)
	}
	if len(actionUuids) > 0 {
		roomData.IgnoreTimer = autoIgnore(actionUuids)
	}
}

func dealNextCard(data *RoomData, nP int) {
	var activePlayerPos = data.CurrentActivePLayer
	var nextPos = -1
	if nP < 0 {
		nextPos = (activePlayerPos + 1 + 4) % 4
	} else {
		nextPos = nP
		data.CurrentActivePLayer = nP
	}
	//var index = 1
	//var highestNum = -1
	//var highestCount = -1
	//var count = getHandsCountValues(data.PlayersByPosition[nextPos].Hands)
	//for n, c := range count {
	//	if c > highestCount {
	//		highestNum = n
	//		highestCount = c
	//	}
	//}
	//for i, c := range data.AllCards {
	//	if c.Num == highestNum {
	//		index = i
	//		break
	//	}
	//}
	var index = 0
	var card = data.AllCards[index]
	data.PlayersByPosition[nextPos].NewCard = card
	data.AllCards = append(data.AllCards[:index], data.AllCards[index+1:]...)
	for i, player := range data.PlayersByPosition {
		if i == nextPos {
			var extra = DealCardExtra{DealCard: card, Position: i}
			var res = ServerResponse{EventName: "actionDealCard", Actions: make([]int, 0), ActionExtra: make([]int, 0), Extra: extra}
			res.Actions, res.ActionExtra = getActionsBeforeChupai(player, player.NewCard)
			serverRespond(UserIdConnMap[player.UserId], res, i)
			data.ChupaiTimer = autoChupai(player.UserId, card.Uid)
		} else {
			var extra = DealCardExtra{Position: nextPos}
			var res = ServerResponse{EventName: "actionDealCard", Actions: make([]int, 0), Extra: extra}
			serverRespond(UserIdConnMap[player.UserId], res, i)
		}
	}
	data.CurrentActivePLayer = nextPos
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
	var allCards = make([]Card, 0, 108)
	var allCardsMap = make(map[int]int)
	var j = 1
	for i := 0; i < 4; i++ {
		for i := WAN_1; i <= WAN_9; i++ {
			var card = Card{Num: i, Uid: j}
			allCardsMap[j] = i
			allCards = append(allCards, card)
			j++
		}
		for i := PIN_1; i <= PIN_9; i++ {
			var card = Card{Num: i, Uid: j}
			allCardsMap[j] = i
			allCards = append(allCards, card)
			j++
		}
		for i := SUO_1; i <= SUO_9; i++ {
			var card = Card{Num: i, Uid: j}
			allCardsMap[j] = i
			allCards = append(allCards, card)
			j++
		}
	}
	rand.Seed(time.Now().UnixNano())
	rand.Shuffle(len(allCards), func(i, j int) { allCards[i], allCards[j] = allCards[j], allCards[i] })
	var laizi = rand.Intn(3)*100 + rand.Intn(9) + 1
	data.Laizi = laizi
	data.AllCards = allCards
	data.AllCardsMap = allCardsMap
}

func initPlayerPosition(data *RoomData) {
	var i = 0
	data.PlayersByPosition = make(map[int]*Player)
	for _, player := range data.Players {
		player.Position = i
		data.PlayersByPosition[i] = player
		i++
	}
}

func sortHands(hands []Card) {
	sort.Slice(hands, func(i, j int) bool {
		return hands[i].Num <= hands[j].Num
	})
}

func addCheckSum(res *ServerResponse, position int) {
	res.CheckSum = ""
}

func serverRespond(s socketio.Conn, res ServerResponse, position int) {
	//addCheckSum(res, position)
	var jsonString, error = json.Marshal(res)
	if error != nil {
		return
	}
	fmt.Println("send to " + strconv.Itoa(position) + string(jsonString))
	//s.Emit(res.EventName, string(jsonString))
	s.Emit(res.EventName, res)
}

func generateNewRoomId() int {
	var res = idStart
	idStart++
	return res
}

func getActionsBeforeChupai(player *Player, newCard Card) ([]int, interface{}) {
	var laizi = roomIdData[UserIdRoomIdMap[player.UserId]].Laizi
	var actions = make(map[int]int)
	var actionExtra = make(map[int]interface{})
	var gang1Extra = make([]int, 0)
	var countValues = getHandsCountValues(player.Hands)
	for k, v := range countValues {
		if v == 3 && k == newCard.Num {
			fmt.Println("can gang")
			fmt.Println(player.Hands)
			fmt.Println(newCard)
			actions[ActionGang1] = 1
			gang1Extra = append(gang1Extra, k)
		}
	}
	if len(gang1Extra) > 0 {
		actionExtra[ActionGang1] = gang1Extra
	}
	var v, ok = player.Outsides[newCard.Num]
	if ok && len(v) == 3 {
		actions[ActionGang2] = 1
	}
	if isHu(player.Hands, newCard.Num, laizi) {
		actions[ActionZimo] = 1
	}
	var result = make([]int, 0)
	for k, _ := range actions {
		result = append(result, k)
	}
	if canLaizi(player.Hands, newCard, roomIdData[UserIdRoomIdMap[player.UserId]].Laizi) {
		result = append(result, ActionLaizi)
	}
	sort.Ints(result)
	return result, actionExtra
}

func getActionsAfterChupai(player *Player, currentActiveCard Card) []int {
	var actions = make(map[int]int)
	var countValues = getHandsCountValues(player.Hands)
	for k, v := range countValues {
		if v >= 2 && currentActiveCard.Num == k {
			actions[ActionPeng] = 1
			if v == 3 {
				actions[ActionGang3] = 1
			}
		}

	}
	//if isHu(player.Hands, currentActiveCard.Num) {
	//	actions[ActionHu] = 1
	//}
	var result = make([]int, 0)
	for k, _ := range actions {
		result = append(result, k)
	}
	sort.Ints(result)
	return result
}

func getHandsCountValues(hands []Card) map[int]int {
	var result = make(map[int]int)
	for _, card := range hands {
		var v = card.Num
		if count, ok := result[v]; ok {
			result[v] = count + 1
		} else {
			result[v] = 1
		}
	}
	return result
}

func actionIgnore(uuid int) {
	var roomId = UserIdRoomIdMap[uuid]
	var roomData = roomIdData[roomId]
	if roomData.WaitActionPlayerCount == 0 {
		return
	}
	roomData.WaitActionPlayerCount -= 1
	var position = roomData.Players[uuid].Position
	roomData.DoneActions[position] = ActionIgnore
	if roomData.WaitActionPlayerCount > 0 {
		return
	}
	decideActions(roomData)
}

func actionPeng(s socketio.Conn, uuid int) {
	var roomId = UserIdRoomIdMap[uuid]
	var roomData = roomIdData[roomId]
	roomData.WaitActionPlayerCount -= 1
	var position = roomData.Players[uuid].Position
	roomData.DoneActions[position] = ActionPeng
	if roomData.WaitActionPlayerCount > 0 {
		return
	}
	decideActions(roomData)
}

func actionGang1(s socketio.Conn, uuid int, num int) {
	defer func() {
		if e := recover(); e != nil {
			fmt.Println("actiongang1 error ")
			fmt.Println(e)
			panic(e)
		}
	}()
	var roomId = UserIdRoomIdMap[uuid]
	var roomData = roomIdData[roomId]
	var self = roomData.Players[uuid]
	if self.NewCard.Num > 0 {
		self.Hands = append(self.Hands, self.NewCard)
	}
	var gangCount = 0
	self.Outsides[num] = make([]Card, 0)
	var extra = Gang1Extra{GangCards: make([]Card, 0), Position: self.Position}
	for i := len(self.Hands) - 1; i >= 0; i-- {
		if self.Hands[i].Num == num {
			gangCount += 1
			self.Outsides[num] = append(self.Outsides[num], self.Hands[i])
			extra.GangCards = append(extra.GangCards, self.Hands[i])
			self.Hands = append(self.Hands[:i], self.Hands[i+1:]...)
		}
		if gangCount == 4 {
			break
		}
	}
	sortHands(self.Hands)
	self.NewCard = Card{}
	var res = ServerResponse{EventName: "actionGang1", Actions: make([]int, 0), Extra: extra}
	for _, player := range roomData.Players {
		serverRespond(UserIdConnMap[player.UserId], res, player.Position)
	}
	dealNextCard(roomData, self.Position)
}

func actionGang2(s socketio.Conn, uuid int) {
	defer func() {
		if e := recover(); e != nil {
			fmt.Println("actiongang2 error ")
			fmt.Println(e)
			panic(e)
		}
	}()
	var roomId = UserIdRoomIdMap[uuid]
	var roomData = roomIdData[roomId]
	var self = roomData.Players[uuid]
	var newCard = self.NewCard
	self.Outsides[newCard.Num] = append(self.Outsides[newCard.Num], newCard)
	self.NewCard = Card{}
	var extra = Gang2Extra{GangCards: make([]Card, 0), Position: self.Position}
	extra.GangCards = append(extra.GangCards, newCard)
	var res = ServerResponse{EventName: "actionGang2", Actions: make([]int, 0), Extra: extra}
	for _, player := range roomData.Players {
		serverRespond(UserIdConnMap[player.UserId], res, player.Position)
	}
	dealNextCard(roomData, self.Position)
}

func actionGang3(s socketio.Conn, uuid int) {
	var roomId = UserIdRoomIdMap[uuid]
	var roomData = roomIdData[roomId]
	roomData.WaitActionPlayerCount -= 1
	var position = roomData.Players[uuid].Position
	roomData.DoneActions[position] = ActionGang3
	if roomData.WaitActionPlayerCount > 0 {
		return
	}
	decideActions(roomData)
}

func actionLaizi(s socketio.Conn, uuid int) {
	var roomId = UserIdRoomIdMap[uuid]
	var roomData = roomIdData[roomId]
	var players = roomData.PlayersByPosition
	var laizi = roomData.Laizi
	var activePlayerIndex = -1
	for _, player := range players {
		if player.UserId == uuid {
			activePlayerIndex = player.Position
			continue
		}
	}

	var extra = LaiziExtra{Position: activePlayerIndex}
	for _, player := range roomData.Players {
		if player.UserId == uuid {
			if player.NewCard.Num > 0 {
				player.Hands = append(player.Hands, player.NewCard)
			}
			for i, c := range player.Hands {
				if c.Num == laizi {
					extra.Card = c
					player.Hands = append(player.Hands[:i], player.Hands[i+1:]...)
					if _, ok := player.Outsides[laizi]; !ok {
						player.Outsides[laizi] = make([]Card, 0)
					}
					player.Outsides[laizi] = append(player.Outsides[laizi], c)
					break
				}
			}
		}
	}

	for _, player := range players {
		var res = ServerResponse{EventName: "actionLaizi", Actions: make([]int, 0), Extra: extra}
		serverRespond(UserIdConnMap[player.UserId], res, player.Position)
	}

	dealNextCard(roomData, activePlayerIndex)
}

func actionZimo(s socketio.Conn, uuid int) {
	var roomId = UserIdRoomIdMap[uuid]
	var roomData = roomIdData[roomId]
	var player = roomData.Players[uuid]
	var extra = ZimoExtra{Position: player.Position, Cards: make([]Card, 0)}
	for _, c := range player.Hands {
		extra.Cards = append(extra.Cards, c)
	}
	extra.Cards = append(extra.Cards, player.NewCard)
	var res = ServerResponse{EventName: "actionZimo", Actions: make([]int, 0), Extra: extra}
	for _, pl := range roomData.Players {
		serverRespond(UserIdConnMap[pl.UserId], res, pl.Position)
	}
	roomData.Winner = player.Position
	gameOver(roomData)
}

func decideActions(roomData *RoomData) {
	var doneActions = roomData.DoneActions
	var finalAction = -1
	var finalActionPos = -1
	for k, v := range doneActions {
		if v > finalAction {
			finalAction = v
			finalActionPos = k
		}
	}
	switch finalAction {
	case ActionIgnore:
		{
			roomData.DoneActions = make(map[int]int)
			dealNextCard(roomData, -1)
			break
		}
	case ActionPeng:
		{
			var extra = PengExtra{Position: finalActionPos, PengCards: make([]Card, 0)}
			var currentActivePlayerPos = roomData.CurrentActivePLayer
			var river = roomData.PlayersByPosition[currentActivePlayerPos].River
			var nextActivePos = finalActionPos
			var nextPlayer = roomData.PlayersByPosition[nextActivePos]
			extra.PengCards = append(extra.PengCards, roomData.CurrentActiveCard)
			for i := len(nextPlayer.Hands) - 1; i >= 0; i-- {
				if nextPlayer.Hands[i].Num == roomData.CurrentActiveCard.Num {
					extra.PengCards = append(extra.PengCards, nextPlayer.Hands[i])
					nextPlayer.Hands = append(nextPlayer.Hands[:i], nextPlayer.Hands[i+1:]...)
				}
				if len(extra.PengCards) == 3 {
					break
				}
			}
			roomData.PlayersByPosition[currentActivePlayerPos].River = river[:len(river)-1]
			roomData.CurrentActivePLayer = nextActivePos
			roomData.CurrentActiveCard = Card{}
			var res = ServerResponse{EventName: "actionPeng", Actions: make([]int, 0), Extra: extra}
			if canLaizi(nextPlayer.Hands, Card{}, roomData.Laizi) {
				res.Actions = append(res.Actions, ActionLaizi)
			}
			for _, player := range roomData.Players {
				serverRespond(UserIdConnMap[player.UserId], res, player.Position)
			}
			break
		}
	case ActionGang3:
		{
			var currentActiveCard = roomData.CurrentActiveCard
			var currentActivePlayerPos = roomData.CurrentActivePLayer
			var river = roomData.PlayersByPosition[currentActivePlayerPos].River
			var nextPos = finalActionPos
			var nextPlayer = roomData.PlayersByPosition[nextPos]
			nextPlayer.Outsides[currentActiveCard.Num] = make([]Card, 0)
			var extra = Gang3Extra{GangCards: make([]Card, 0), Position: nextPos}
			extra.GangCards = append(extra.GangCards, currentActiveCard)
			for i := len(nextPlayer.Hands) - 1; i >= 0; i-- {
				if nextPlayer.Hands[i].Num == currentActiveCard.Num {
					extra.GangCards = append(extra.GangCards, nextPlayer.Hands[i])
					nextPlayer.Hands = append(nextPlayer.Hands[:i], nextPlayer.Hands[i+1:]...)
				}
				if len(extra.GangCards) == 4 {
					break
				}
			}
			sortHands(nextPlayer.Hands)
			roomData.PlayersByPosition[currentActivePlayerPos].River = river[:len(river)-1]
			roomData.CurrentActivePLayer = nextPos
			roomData.CurrentActiveCard = Card{}
			var res = ServerResponse{EventName: "actionGang3", Actions: make([]int, 0), Extra: extra}
			for _, player := range roomData.Players {
				serverRespond(UserIdConnMap[player.UserId], res, player.Position)
			}
			dealNextCard(roomData, nextPos)
			break
		}

	}
	clearIgnoreTimer(roomData)
}

func canLaizi(hands []Card, newCard Card, laizi int) bool {
	for _, c := range hands {
		if c.Num == laizi {
			return true
		}
	}
	if newCard.Num == laizi {
		return true
	}
	return false
}

func gameOver(roomData *RoomData) {
	var extra = GameOverExtra{Winner: roomData.PlayersByPosition[roomData.Winner].Name}
	var res = ServerResponse{EventName: "actionGameOver", Extra: extra}
	for _, player := range roomData.Players {
		serverRespond(UserIdConnMap[player.UserId], res, player.Position)
	}
	resetRoomData(roomData)
}

func resetRoomData(roomData *RoomData) {
	roomData.AllCards = make([]Card, 0)
	roomData.Laizi = 0
	roomData.AllCardsMap = nil
	roomData.CurrentActiveCard = Card{}
	roomData.CurrentActivePLayer = -1
	roomData.CurrentState = 0
	roomData.WaitActionPlayerCount = 0
	roomData.DoneActions = nil
	roomData.Winner = -1
	roomData.PlayersByPosition = nil
	for _, player := range roomData.Players {
		player.Position = -1
		player.Hands = make([]Card, 0)
		player.Outsides = make(map[int][]Card, 0)
		player.River = make([]Card, 0)
		player.ReadyState = 0
		player.NewCard = Card{}
	}
}

func getUserFromSessionId(sessionId string) User {
	var seid = "session_" + sessionId
	var db = GetDb()
	var rdb = GetRedis()
	var ctx = context.Background()

	val, _ := rdb.Get(ctx, seid).Result()

	var result = map[interface{}]interface{}{}
	dec := gob.NewDecoder(bytes.NewBuffer([]byte(val)))
	var err = dec.Decode(&result)
	if err != nil {
		fmt.Println(err)
	}
	var uuid = int(result["uuid"].(int32))
	var user User
	db.Table("users").Select("id", "name").Where(map[string]interface{}{"id": uuid}).Take(&user)
	return user
}
