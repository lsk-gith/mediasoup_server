package main

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/websocket"
	"log"
	"mediasoup_server/bean"
	"mediasoup_server/common_util"
	"mediasoup_server/worker"
	"net/http"
	"time"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
	HandshakeTimeout: 10 * time.Second,
	ReadBufferSize:   1024,
	WriteBufferSize:  1024,
}

func handleSignal(message []byte, conn *websocket.Conn) error {
	var msg bean.SignalMessage
	if err := json.Unmarshal(message, &msg); err != nil {
		return err
	}
	var err error
	switch msg.Type {
	case "offer":
		err = worker.ProcessUpOfferSdp(message, conn)
		if err != nil {
			fmt.Printf("ProcessUpOfferSdp err:%v \n", err)
		}
	case "answer":
		err = worker.ProcessDownAnswerSdp(message, conn)
		if err != nil {
			fmt.Printf("ProcessDownAnswerSdp err:%v \n", err)
		}
	case "heatBeat":
		conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
		resp := map[string]string{}
		resp["type"] = "heatBeat"
		marshal, _ := json.Marshal(resp)
		conn.WriteMessage(websocket.TextMessage, marshal)
	case "hangup":
		fmt.Println("hangup")
	case "offer_pipe_01":
		err = worker.ProcessPipe01(message, conn)
		if err != nil {
			fmt.Printf("ProcessUpOfferSdp err:%v \n", err)
		}
	case "offer_pipe_02":
		err = worker.ProcessPipe02(message, conn)
		if err != nil {
			fmt.Printf("ProcessUpOfferSdp err:%v \n", err)
		}
	case "answer_pipe_02":
		err = worker.ProcessAnswerPipe02(message, conn)
		if err != nil {
			fmt.Printf("ProcessUpOfferSdp err:%v \n", err)
		}
	case "signal":
		err = worker.ProcessSignal(message, conn)
		if err != nil {
			fmt.Printf("ProcessUpOfferSdp err:%v \n", err)
		}
	default:
		return err
	}

	if err != nil {
		return err
	}

	return nil
}

func handleWebSocket(w http.ResponseWriter, r *http.Request) {

	// 升级HTTP连接为WebSocket
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v\n", err)
		return
	}
	defer func() {
		handleDisconnect(conn) // Call disconnect handler
		conn.Close()
	}()
	// 设置读写超时
	conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	conn.SetWriteDeadline(time.Now().Add(10 * time.Second))

	// 设置连接参数
	conn.SetReadLimit(512 * 1024) // 512KB

	// 处理ping/pong
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	// 启动ping协程
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if err := conn.WriteControl(websocket.PingMessage, []byte{}, time.Now().Add(10*time.Second)); err != nil {
					log.Printf("Ping error: %v", err)
					return
				}
			}
		}
	}()

	// 创建一个done通道用于关闭ping协程
	done := make(chan struct{})
	defer close(done)

	for {
		// 重置读取超时
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		_, message, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("Unexpected close error: %v", err)
			} else {
				log.Printf("Read error: %v", err)
			}
			return
		}

		log.Printf("Received message from client: %s\n", message)

		// 处理信令消息
		err = handleSignal(message, conn)
		if err != nil {
			log.Printf("Signal handling error: %v\n", err)

			continue
		}
	}
}

func handleDisconnect(conn *websocket.Conn) {
	log.Printf("Client disconnected: %v, port:\n", conn.RemoteAddr())
	worker.ClearUserInfo(conn)

}

func main() {
	ips, err := common_util.GetLocalIPs()
	if err != nil {
		fmt.Printf("Get local ip error: %v\n", err)
		return
	}
	if len(ips) == 0 {
		fmt.Printf("Get local ip is null: %v\n", err)
		return
	}
	LocalIp := ips[0]
	worker.StartWorker(LocalIp)
	http.HandleFunc("/ws", handleWebSocket)

	// 启动HTTP服务器
	port := 8080
	fmt.Printf("Starting WebSocket server on port %d...\n", port)
	if err := http.ListenAndServe(fmt.Sprintf(":%d", port), nil); err != nil {
		log.Fatal("ListenAndServe error:", err)
	}

}
