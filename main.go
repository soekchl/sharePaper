package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"

	. "github.com/soekchl/myUtils"
	"github.com/soekchl/websocket"
	// "code.google.com/p/go.net/websocket"
)

/*
	---Cmd List---
	server->client
		1-init
		2-edit paper
		3-online count
		4-lock paper
		5-unlock paper
	client->server
		2-edit paper
		4-lock paper
		5-unlock paper
*/
type Message struct {
	Cmd   int    `json:"cmd"`
	Index int    `json:"index"`
	Data  string `json:"data"`
	ws    *websocket.Conn
}

type Paper struct {
	Data  string    `json:"data"` // paper data save momery
	Lock  bool      `json:"lock"` // 是否锁定
	id    int       // paper id - key
	mTime time.Time // last edit time
	ip    string    // last edit ip
}

//全局信息
var (
	users         []*websocket.Conn
	allCount      = 0
	userLock      = make(map[int]*Paper) // 一个连接同时只能锁定一个 连接断开时解锁
	paperMap      = make(map[int]*Paper)
	paperMapMutex sync.RWMutex
	sendMsg       chan *Message
	serverPort    = ":8080"
)

func init() {
	sendMsg = make(chan *Message, 10)
	if len(os.Args) > 1 {
		serverPort = os.Args[1]
	}
	paperMap[-1] = &Paper{
		Data: `
v0.2.1 更新版本

增加了用户唯一编辑

当一个用户编辑的时候其他用户页面当前编辑框不可编辑

后续会增加
1、加密传输
2、隐私编辑框（需使用密码编辑和查看）
3、redis存储
`,
		Lock: true,
	}
}

func main() {
	//绑定效果页面
	http.HandleFunc("/", index)
	// 监听页面
	http.HandleFunc("/monitor", monitor)
	//绑定socket方法
	http.Handle("/webSocket", websocket.Handler(webSocket))
	Notice("Listen ", GetIp(), serverPort)
	go sendServer()
	//开始监听
	http.ListenAndServe(serverPort, nil)
}

func index(w http.ResponseWriter, r *http.Request) {
	Debugf("index ip=%v", r.RemoteAddr)
	http.ServeFile(w, r, "index.html")
}

func monitor(w http.ResponseWriter, r *http.Request) {
	Debugf("monitor ip=%v", r.RemoteAddr)
	paperMapMutex.RLock()
	defer paperMapMutex.RUnlock()
	str := "----monitor----\n\n"
	for k, v := range paperMap {
		str += fmt.Sprintf("index:%v\neditTime:%v\nip:%v\ndata:%v\nlock:%v\n\n\n",
			k,
			v.mTime.Format("2006-01-02 15:04:05.000"),
			v.ip,
			v.Data,
			v.Lock,
		)
		str += "-------------------------------------------------"
	}

	w.Write([]byte(str))
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
		if msg.Cmd == 2 || msg.Cmd == 4 || msg.Cmd == 5 {
			msg.ws = ws
			// Debugf("cmd=%v id=%v", msg.Cmd, index)
			paperSet(msg, ws.RemoteAddr().String(), index)
			sendMsg <- msg
		}
	}
	//	close
	changeOnline(-1)
	clearPaper(index)
	users[index] = nil
}

func paperSet(msg *Message, ip string, index int) {
	paperMapMutex.Lock()
	defer paperMapMutex.Unlock()
	tmp, ok := paperMap[msg.Index]
	if !ok {
		tmp = &Paper{id: msg.Index}
	}
	if msg.Cmd == 2 && len(msg.Data) < 1 {
		delete(paperMap, msg.Index)
		return
	}
	if msg.Cmd == 2 {
		tmp.Data = msg.Data
	} else if msg.Cmd == 4 || msg.Cmd == 5 {
		tmp.Lock = msg.Cmd == 4
		clearPaper(index)
		userLock[index] = tmp
	}
	tmp.ip = ip
	tmp.mTime = time.Now()
	paperMap[msg.Index] = tmp
	// Warnf("%#v", tmp)
}

func clearPaper(index int) {
	lp, ok := userLock[index]
	if ok {
		lp.Lock = false // 解锁
		sendMsg <- &Message{Cmd: 5, Index: lp.id}
		delete(userLock, index)
	}
}

func sendServer() {
	var m *Message
	for m = range sendMsg {
		send(m)
	}
}

// send online count edit
func changeOnline(value int) {
	allCount += value
	Debugf("changeOnline online=%v value=%v", allCount, value)
	send(&Message{
		Cmd:  3,
		Data: fmt.Sprint(allCount),
	})
}

func sendInitData(ws *websocket.Conn) {
	paperMapMutex.RLock()
	defer paperMapMutex.RUnlock()

	mbuff, err := json.Marshal(&paperMap)
	if err != nil {
		Error(err)
		return
	}

	buff, err := json.Marshal(&Message{
		Cmd:  1,
		Data: string(mbuff),
	})
	if err != nil {
		Error(err)
		return
	}
	Debug(string(buff))
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
