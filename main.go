package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	. "github.com/soekchl/myUtils"
	"github.com/soekchl/webServer/src/common/config"
	"github.com/soekchl/websocket"
	// "code.google.com/p/go.net/websocket"
)

/*
	---Cmd List---
	server->client
		1-init		-	not used
		2-edit paper
		3-online count
	client->server
		1-init		-	not used
		2-edit paper
*/
type Message struct {
	Cmd  int    `json:"cmd"`
	Data string `json:"data"`
	ws   *websocket.Conn
}

type Paper struct {
	data  string    // paper data save momery
	mTime time.Time // last edit time
	ip    string    // last edit ip
	count int       // online count
}

//全局信息
var users []*websocket.Conn
var paper *Paper
var sendMsg chan *Message

func init() {
	configName := "./config/config.ini"
	config.Config(configName)

	paper = &Paper{mTime: time.Now()}
	sendMsg = make(chan *Message, 10)
}

func main() {
	//绑定效果页面
	http.HandleFunc("/", index)
	//绑定socket方法
	http.Handle("/webSocket", websocket.Handler(webSocket))
	Notice("Listen ", config.GetString("server.port"))
	go sendServer()
	//开始监听
	http.ListenAndServe(config.GetString("server.port"), nil)
}

func index(w http.ResponseWriter, r *http.Request) {
	Debugf("index ip=%v", r.RemoteAddr)
	http.ServeFile(w, r, "index.html")
}

func webSocket(ws *websocket.Conn) {
	Debugf("websocket ip=%v", ws.RemoteAddr().String())
	// save webSocket List
	index := addUsers(ws)
	changeOnline(1)
	sendInitData(ws)
	var err error
	// receive
	var buff string
	for {
		err = websocket.Message.Receive(ws, &buff)
		// Debug("data：", buff)
		if err != nil {
			//移除出错的链接
			break
		}

		msg := &Message{}
		err = json.Unmarshal([]byte(buff), msg)
		if err != nil {
			Errorf("解析数据异常... err=%v data=%v", err, buff)
			break
		}
		if msg.Cmd == 2 {
			msg.ws = ws
			paper.data = msg.Data
			sendMsg <- msg
			paper.ip = ws.RemoteAddr().String()
			paper.mTime = time.Now()
		}
	}
	//	close
	changeOnline(-1)
	users[index] = nil
}

func sendServer() {
	var m *Message
	for m = range sendMsg {
		switch m.Cmd {
		case 2: // edit paper data
			send(m)
		}
	}
}

// send online count edit
func changeOnline(value int) {
	paper.count += value
	Debugf("changeOnline online=%v value=%v", paper.count, value)
	send(&Message{
		Cmd:  3,
		Data: fmt.Sprint(paper.count),
	})
}

func sendInitData(ws *websocket.Conn) {
	buff, err := json.Marshal(&Message{
		Cmd:  2,
		Data: paper.data,
	})
	if err != nil {
		Error(err)
		return
	}
	websocket.Message.Send(ws, string(buff))
}

func send(msg *Message) {
	buff, err := json.Marshal(msg)
	if err != nil {
		Error(err)
		return
	}

	for _, k := range users {
		if k == nil {
			continue
		}
		if msg.ws == k { // not send me
			continue
		}
		err = websocket.Message.Send(k, string(buff))
		if err != nil {
			Error(err)
		}
	}
}

func addUsers(ws *websocket.Conn) int {
	for k, v := range users {
		if v == nil {
			users[k] = ws
			return k
		}
	}
	users = append(users, ws)
	return len(users) - 1
}
