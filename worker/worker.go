package worker

import (
	"bufio"
	"encoding/json"
	"fmt"
	"github.com/gorilla/websocket"
	"io"
	"log"
	"mediasoup_server/bean"
	"mediasoup_server/common_util"
	"net"
	"os"
	"os/exec"
	"runtime/debug"
	"strconv"
	"sync"
	"syscall"
	"time"
)

var workerManager *WorkerManager

var (
	roomMap           = sync.Map{}
	shaMap            = make(map[string]string)
	localIp           string
	udpPort           int
	pipe02_ssrc_audio uint32
	pipe02_ssrc_video uint32
)

func StartWorker(ip string) {
	udpPort = 44445
	localIp = ip
	fmt.Printf("localIp: %s\n", localIp)
	workerManager = StartMediasoupWorker()
	var msg = strconv.FormatInt(common_util.GneId(), 10) + ":worker.createWebRtcServer::{\"listenInfos\":[{\"protocol\":\"udp\",\"ip\":\"" + localIp + "\",\"port\":" + strconv.FormatInt(int64(udpPort), 10) + "},{\"protocol\":\"tcp\",\"ip\":\"" + localIp + "\",\"port\":" + strconv.FormatInt(int64(udpPort), 10) + "}],\"webRtcServerId\":\"001\"}"
	workerMsgBytes := []byte(msg + "\000")
	raw := common_util.Encode(workerMsgBytes)
	if _, err := workerManager.ProducerSocket.Write(raw); err != nil {
		fmt.Printf("workerManager.ProducerSocket.Write err:%s\n", err)
	}
	log.Printf("Sending message to worker: %s", string(raw))
}

type WorkerManager struct {
	ProducerSocket net.Conn // 用于发送数据到子进程
	ConsumerSocket net.Conn // 用于接收子进程的响应
	closed         bool
	mutex          sync.Mutex
}

func NewWorkerManager(producerConn, consumerConn net.Conn) *WorkerManager {
	wm := &WorkerManager{
		ProducerSocket: producerConn,
		ConsumerSocket: consumerConn,
		closed:         false,
	}
	go wm.readLoop()
	return wm
}

func (wm *WorkerManager) Close() {
	wm.mutex.Lock()
	defer wm.mutex.Unlock()
	if !wm.closed {
		wm.closed = true
		wm.ProducerSocket.Close()
		wm.ConsumerSocket.Close()
	}
}

func (wm *WorkerManager) processNSPayload(nsPayload []byte) {
	switch nsPayload[0] {
	case '{':
		if err := ProcessWorkerMessage(nsPayload); err != nil {
			fmt.Printf("processMessage err:%v", err)
		}
	default:
		fmt.Printf("[processNSPayload] unexpected data:%s", string(nsPayload))
	}
}
func (wm *WorkerManager) readLoop() {
	decoder := common_util.NewDecoder()
	go func() {
		if err := recover(); err != nil {
			fmt.Printf("channel panic, err:%v stack:%s, time:%v\n", err, debug.Stack(), time.Now())
		}
		for nsPayload := range decoder.Result() {
			wm.processNSPayload(nsPayload)
		}
	}()
	buf := make([]byte, 4194304)
	for {
		n, err := wm.ConsumerSocket.Read(buf)
		if err != nil {
			fmt.Printf("rcv from worker, read error: %v", err)
		}
		data := buf[:n]
		decoder.Feed(data)
	}
	wm.Close()
}

func StartMediasoupWorker() *WorkerManager {
	// 创建socketpair
	producerPair, err := createSocketPair()
	if err != nil {
		log.Printf("Failed to create producer pair: %v", err)
		return nil
	}
	consumerPair, err := createSocketPair()
	if err != nil {
		log.Printf("Failed to create consumer pair: %v", err)
		return nil
	}
	producerSocket, err := fileToConn(producerPair[0])
	if err != nil {
		log.Printf("Failed to create producer socket: %v", err)
		return nil
	}
	consumerSocket, err := fileToConn(consumerPair[0])
	if err != nil {
		log.Printf("Failed to create consumer socket: %v", err)
		return nil
	}
	// mediasoup-worker的参数
	args := []string{
		"--logLevel=debug",
		"--logTag=info",
		"--logTag=ice",
		"--logTag=dtls",
		"--logTag=rtp",
		"--logTag=srtp",
		"--logTag=rtcp",
		"--logTag=rtx",
		"--logTag=bwe",
		"--logTag=score",
		"--logTag=simulcast",
		"--logTag=svc",
		"--logTag=sctp",
		"--logTag=message",
		"--logTag=rtmp",
		"--logTag=fec",
		"--logTag=http_client",
		"--rtcMinPort=40000",
		"--rtcMaxPort=49999",
	}
	cmd := exec.Command("./mediasoup-3.12.16/worker/out/Release/mediasoup-worker", args...)
	cmd.ExtraFiles = []*os.File{producerPair[1], consumerPair[1]}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		fmt.Printf("[newWorker] stdout. err:%v", err)
		return nil
	}
	var stderr io.ReadCloser
	stderr, err = cmd.StderrPipe()
	if err != nil {
		fmt.Printf("[newWorker] stderr. err:%v", err)
		return nil
	}
	err = cmd.Start()
	if err != nil {
		log.Fatalf("Failed to start mediasoup-worker: %v", err)
	}
	producerPair[0].Close()
	consumerPair[0].Close()
	pid := cmd.Process.Pid
	go func() {
		r := bufio.NewReader(stderr)
		for {
			line, _, err := r.ReadLine()
			if err != nil {
				break
			}
			if len(line) > 0 {
				fmt.Printf("child[%d] worker stderr line:%s", pid, line)
			}
		}
	}()
	go func() {
		r := bufio.NewReader(stdout)
		for {
			line, _, err := r.ReadLine()
			if err != nil {
				break
			}
			if len(line) > 0 {
				fmt.Printf("child[%d] worker stdout line:%s", pid, line)
			}
		}
	}()
	workerManager := NewWorkerManager(producerSocket, consumerSocket)
	go func() {
		err := cmd.Wait()
		if err != nil {
			log.Printf("mediasoup-worker exited with error: %v", err)
		} else {
			log.Printf("mediasoup-worker exited normally")
		}
		workerManager.Close()
	}()
	return workerManager
}

func fileToConn(file *os.File) (net.Conn, error) {
	conn, err := net.FileConn(file)
	if err != nil {
		file.Close()
		return nil, err
	}
	return conn, nil
}

func createSocketPair() (file [2]*os.File, err error) {
	fd, err := syscall.Socketpair(syscall.AF_LOCAL, syscall.SOCK_STREAM, 0)
	if err != nil {
		fmt.Println("[createSocketPair] syscall.socketpair failed. err:%v", err)
		return file, err
	}
	file[0] = os.NewFile(uintptr(fd[0]), "")
	file[1] = os.NewFile(uintptr(fd[1]), "")
	return
}

func ProcessWorkerMessage(nsPayload []byte) error {
	parseWorkerMsg(nsPayload)
	return nil
}

func parseWorkerMsg(payload []byte) {
	payloadCopy := make([]byte, len(payload))
	copy(payloadCopy, payload)
	go func() {
		fmt.Printf("worker payload:%s\n", string(payloadCopy))
		response(payloadCopy)
	}()
}

func response(payload []byte) {
	var workerMsg map[string]interface{}
	err := json.Unmarshal(payload, &workerMsg)
	if err != nil {
		return
	}
	if _, exist := workerMsg["targetId"]; exist {
		if condition, ex := workerMsg["event"].(string); ex {
			if condition == "running" {
				data, ok := workerMsg["data"]
				if !ok {
					panic("worker data not found")
					return
				}
				bytes, err := json.Marshal(data)
				if err != nil {
					panic("worker data marshal err")
				}
				shas := make([]bean.Sha, 5)
				err = json.Unmarshal(bytes, &shas)
				if err != nil {
					return
				}
				for _, sha := range shas {
					shaMap[sha.Algorithm] = sha.Value
				}
				fmt.Printf("shaMap: %s\n", shaMap)
				return
			}
		}
	}
	if id, exist := workerMsg["id"].(float64); exist {
		workId := int64(id)
		if workId == 1 {
			data, ok := workerMsg["data"]
			if !ok {
				panic("worker data not found")
				return
			}
			bytes, err := json.Marshal(data)
			if err != nil {
				panic("worker data marshal err")
			}
			shas := make([]bean.Sha, 5)
			err = json.Unmarshal(bytes, &shas)
			if err != nil {
				return
			}
			for _, sha := range shas {
				shaMap[sha.Algorithm] = sha.Value
			}
			fmt.Printf("shaMap: %s\n", shaMap)
			return
		}
	}
}

func ProcessUpAnswerSdp(user *bean.User, offerSdp string) {
	var answer = map[string]string{}
	sdp := common_util.GenerateUpAnswerSDP("sha-512", shaMap["sha-512"], user, offerSdp)
	answer["sdp"] = sdp
	answer["type"] = "answer"
	conn := user.Conn
	var answerSdp = make(map[string]interface{})
	answerSdp["type"] = "answer"
	answerSdp["answer"] = answer
	marshal, _ := json.Marshal(answerSdp)
	conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	conn.WriteMessage(websocket.TextMessage, marshal)
}

func ProcessDownAnswerSdp(message []byte, conn *websocket.Conn) error {
	var answer bean.AnswerReq
	err := json.Unmarshal(message, &answer)
	if err != nil {
		fmt.Printf("[ProcessDownAnswerSdp] json.Unmarshal err:%v\n", err)
		return err
	}
	userId := answer.UserId
	roomId := answer.RoomId
	roomValue, exist := roomMap.Load(roomId)
	if !exist {
		return fmt.Errorf("roomId: %d not exist", roomId)
	}
	room := roomValue.(*bean.Room)
	value, ok := room.UserIdUserMap.Load(userId)
	if !ok {
		return fmt.Errorf("userId:%v not exist", userId)
	}
	user := value.(*bean.User)
	user.Mu.Lock()
	defer user.Mu.Unlock()
	//1、需要解析出来，dtls参数给worker实现connect
	name, content := common_util.GetFingerNameContent(answer.Answer.Sdp)
	producerTransportId := user.Consumer.TransportId
	id := common_util.GneId()
	idStr := strconv.FormatInt(id, 10)
	msg := idStr + ":transport.connect:" + producerTransportId + ":{\"dtlsParameters\":{\"fingerprints\": [{\"algorithm\":\"" + name + "\",\"value\":\"" + content + "\"}],\"role\":\"client\"}}"
	workerMsgBytes := []byte(msg + "\000")
	raw := common_util.Encode(workerMsgBytes)
	if _, err = workerManager.ProducerSocket.Write(raw); err != nil {
		return err
	}
	//2、需要向worker发起consume请求 该步骤应该在transport connection之后再transport.consume
	remoteConsumer := user.Consumer.Consumers
	producerId := ""
	consumerId := ""
	for _, consumers := range remoteConsumer {
		for _, consumer := range consumers {
			transType := "simple"
			for _, encoding := range consumer.ConsumableRtpEncodings {
				if encoding.ScalabilityMode != "" {
					transType = "simulcast"
				}
			}
			consumerId = consumer.ConsumerId
			producerId = consumer.ProducerId
			kind := consumer.Kind
			encodings := consumer.ConsumableRtpEncodings
			parameters := consumer.RtpParameters
			req := make(map[string]interface{})
			req["consumerId"] = consumerId
			req["producerId"] = producerId
			req["kind"] = kind
			req["type"] = transType
			req["rtpParameters"] = parameters
			req["paused"] = true
			req["consumableRtpEncodings"] = encodings
			req["ignoreDtx"] = false
			marshal, _ := json.Marshal(req)
			id = common_util.GneId()
			idStr = strconv.FormatInt(id, 10)
			msg = idStr + ":transport.consume:" + producerTransportId + ":" + string(marshal)
			workerMsgBytes = []byte(msg + "\000")
			raw = common_util.Encode(workerMsgBytes)
			if _, err = workerManager.ProducerSocket.Write(raw); err != nil {
				return err
			}
			id = common_util.GneId()
			idStr = strconv.FormatInt(id, 10)
			msg = idStr + ":consumer.resume:" + consumerId + ":{}"
			workerMsgBytes = []byte(msg + "\000")
			raw = common_util.Encode(workerMsgBytes)
			if _, err = workerManager.ProducerSocket.Write(raw); err != nil {
				return err
			}
		}
	}

	return nil
}

func ProcessUpOfferSdp(message []byte, conn *websocket.Conn) error {
	var offer bean.OfferReq
	err := json.Unmarshal(message, &offer)
	if err != nil {
		return err
	}
	var activeSpeakerObserverId string
	var audioLevelObserverId string
	roomId := offer.RoomId
	userId := offer.UserId
	fmt.Printf("offer:\n %v\n", offer.Offer.Sdp)
	value, ok := roomMap.Load(roomId)
	var room *bean.Room
	user := &bean.User{
		Conn:                   conn,
		UserId:                 userId,
		RoomId:                 roomId,
		WorkerIdTransportIdMap: make(map[int64]string), // 初始化新映射
	}
	//房间不存在
	if !ok {
		id := common_util.GneId()
		idStr := strconv.FormatInt(id, 10)
		msg := idStr + ":worker.createRouter:req-123:{\"routerId\":\"" + roomId + "\"}"
		workerMsgBytes := []byte(msg + "\000")
		raw := common_util.Encode(workerMsgBytes)
		if _, err := workerManager.ProducerSocket.Write(raw); err != nil {
			return err
		}
		audioLevelObserverId = common_util.RandStringBytes(10)
		id = common_util.GneId()
		idStr = strconv.FormatInt(id, 10)
		msg = idStr + ":router.createAudioLevelObserver:" + roomId + ":{\"rtpObserverId\":\"" + audioLevelObserverId + "\",\"maxEntries\":1,\"threshold\":-80,\"interval\":800}"
		workerMsgBytes = []byte(msg + "\000")
		raw = common_util.Encode(workerMsgBytes)
		if _, err = workerManager.ProducerSocket.Write(raw); err != nil {
			return err
		}
		log.Printf("Sending message to worker: %s", string(raw))
		activeSpeakerObserverId = common_util.RandStringBytes(10)
		id = common_util.GneId()
		idStr = strconv.FormatInt(id, 10)
		msg = idStr + ":router.createActiveSpeakerObserver:" + roomId + ":{\"rtpObserverId\":\"" + activeSpeakerObserverId + "\",\"interval\":300}"
		workerMsgBytes = []byte(msg + "\000")
		raw = common_util.Encode(workerMsgBytes)
		if _, err = workerManager.ProducerSocket.Write(raw); err != nil {
			return err
		}
		userIdUserMap := sync.Map{}
		userIdUserMap.Store(userId, user)
		room = &bean.Room{
			ActiveSpeakerObserverId: audioLevelObserverId,
			AudioLevelObserverId:    activeSpeakerObserverId,
			UserIdUserMap:           userIdUserMap,
		}
		roomMap.Store(roomId, room)
	} else {
		//房间存在
		room = value.(*bean.Room) // 断言为指针类型
		fmt.Printf("roomId:%v alread exist", roomId)
		activeSpeakerObserverId = room.ActiveSpeakerObserverId
		audioLevelObserverId = room.AudioLevelObserverId
		room.UserIdUserMap.Store(userId, user)
	}
	user.Mu.Lock()
	defer user.Mu.Unlock()
	fmt.Printf("roomId:%v userId:%v\n", roomId, userId)
	producerTransportId := common_util.RandStringBytes(10)
	id := common_util.GneId()
	idStr := strconv.FormatInt(id, 10)
	user.WorkerIdTransportIdMap[id] = producerTransportId
	producerFinger := common_util.RandomStringWithHyphen(32)
	producerPassword := common_util.RandomStringWithHyphen(32)
	msg := idStr + ":router.createWebRtcTransportWithServer:" + roomId + ":{\"transportId\":\"" + producerTransportId + "\",\"finger\":\"" + producerFinger + "\",\"password\":\"" + producerPassword + "\",\"listenIps\":[{\"ip\":\"" + localIp + "\"}],\"port\":" + strconv.FormatInt(int64(udpPort), 10) + ",\"webRtcServerId\":\"001\",\"enableUdp\":true,\"enableTcp\":false,\"preferUdp\":false,\"preferTcp\":false,\"initialAvailableOutgoingBitrate\":1000000,\"enableSctp\":true,\"numSctpStreams\":{\"OS\":1024,\"MIS\":1024},\"maxSctpMessageSize\":262144,\"sctpSendBufferSize\":262144,\"isDataChannel\":true}"
	workerMsgBytes := []byte(msg + "\000")
	raw := common_util.Encode(workerMsgBytes)
	if _, err = workerManager.ProducerSocket.Write(raw); err != nil {
		return err
	}
	producerTransport := bean.ProducerTransport{
		TransportId:    producerTransportId,
		Ip:             localIp,
		Port:           udpPort,
		Fingerprint:    producerFinger,
		FingerPassword: producerPassword,
		Producers:      make(map[string]bean.MediaParameters),
	}
	user.Producer = producerTransport

	id = common_util.GneId()
	idStr = strconv.FormatInt(id, 10)
	msg = idStr + ":transport.enableTraceEvent:" + producerTransportId + ":{\"types\":[\"bwe\"]}"
	workerMsgBytes = []byte(msg + "\000")
	raw = common_util.Encode(workerMsgBytes)
	if _, err = workerManager.ProducerSocket.Write(raw); err != nil {
		return err
	}
	id = common_util.GneId()
	idStr = strconv.FormatInt(id, 10)
	msg = idStr + ":transport.setMaxIncomingBitrate:" + producerTransportId + ":{\"bitrate\":1500000}"
	workerMsgBytes = []byte(msg + "\000")
	raw = common_util.Encode(workerMsgBytes)
	if _, err = workerManager.ProducerSocket.Write(raw); err != nil {
		return err
	}
	name, content := common_util.GetFingerNameContent(offer.Offer.Sdp)

	id = common_util.GneId()
	idStr = strconv.FormatInt(id, 10)
	msg = idStr + ":transport.connect:" + producerTransportId + ":{\"dtlsParameters\":{\"fingerprints\": [{\"algorithm\":\"" + name + "\",\"value\":\"" + content + "\"}],\"role\":\"auto\"}}"
	workerMsgBytes = []byte(msg + "\000")
	raw = common_util.Encode(workerMsgBytes)
	if _, err = workerManager.ProducerSocket.Write(raw); err != nil {
		return err
	}
	audioParams, videoParams, err := common_util.ParseUpOfferSDP(offer.Offer.Sdp)
	if err != nil {
		return err
	}
	audioProduceId := common_util.RandStringBytes(10)
	videoProduceId := common_util.RandStringBytes(10)
	audioParams.ProducerId = audioProduceId
	videoParams.ProducerId = videoProduceId
	producerTransport.Producers[audioProduceId] = audioParams
	producerTransport.Producers[videoProduceId] = videoParams
	audio, err := json.Marshal(audioParams)
	if err != nil {
		return err
	}
	video, err := json.Marshal(videoParams)
	if err != nil {
		return err
	}
	id = common_util.GneId()
	idStr = strconv.FormatInt(id, 10)
	msg = idStr + ":transport.produce:" + producerTransportId + ":" + string(audio)
	workerMsgBytes = []byte(msg + "\000")
	raw = common_util.Encode(workerMsgBytes)
	if _, err = workerManager.ProducerSocket.Write(raw); err != nil {
		return err
	}

	id = common_util.GneId()
	idStr = strconv.FormatInt(id, 10)
	msg = idStr + ":rtpObserver.addProducer:" + audioLevelObserverId + ":{\"producerId\":\"" + audioProduceId + "\"}"
	workerMsgBytes = []byte(msg + "\000")
	raw = common_util.Encode(workerMsgBytes)
	if _, err = workerManager.ProducerSocket.Write(raw); err != nil {
		return err
	}

	id = common_util.GneId()
	idStr = strconv.FormatInt(id, 10)
	msg = idStr + ":transport.produce:" + producerTransportId + ":" + string(video)
	workerMsgBytes = []byte(msg + "\000")
	raw = common_util.Encode(workerMsgBytes)
	if _, err = workerManager.ProducerSocket.Write(raw); err != nil {
		return err
	}
	time.Sleep(20 * time.Millisecond)
	//上行answer
	ProcessUpAnswerSdp(user, offer.Offer.Sdp)
	//下行订阅
	time.Sleep(20 * time.Millisecond)
	ProcessDownOfferSdp(roomId)
	return nil
}

func ProcessDownOfferSdp(roomId string) error {
	roomValue, exit := roomMap.Load(roomId)
	if !exit {
		return fmt.Errorf("roomId %s not exist", roomId)
	}
	room := roomValue.(*bean.Room)
	roomUsers := make([]*bean.User, 0)
	room.UserIdUserMap.Range(func(key, value any) bool {
		user := value.(*bean.User)
		roomUsers = append(roomUsers, user)
		return true
	})
	if len(roomUsers) <= 1 {
		return nil
	}
	for index, notifyUser := range roomUsers {
		notifyUserId := notifyUser.UserId
		conn := notifyUser.Conn
		users := make([]*bean.User, 0)
		room.UserIdUserMap.Range(func(key, value any) bool {
			user := value.(*bean.User)
			if user.UserId != notifyUserId {
				users = append(users, user)
			}
			return true
		})
		//先关闭之前的流
		transportId := notifyUser.Consumer.TransportId
		for _, consumers := range notifyUser.Consumer.Consumers {
			for consumerId, _ := range consumers {
				id := common_util.GneId()
				idStr := strconv.FormatInt(id, 10)
				msg := idStr + ":transport.closeConsumer:" + transportId + ":{\"consumerId\":\"" + consumerId + "\"}"
				workerMsgBytes := []byte(msg + "\000")
				raw := common_util.Encode(workerMsgBytes)
				if _, err := workerManager.ProducerSocket.Write(raw); err != nil {
					return err
				}
			}
		}
		if transportId != "" {
			id := common_util.GneId()
			idStr := strconv.FormatInt(id, 10)
			msg := idStr + ":router.closeTransport:" + roomId + ":{\"transportId\":\"" + transportId + "\"}"
			workerMsgBytes := []byte(msg + "\000")
			raw := common_util.Encode(workerMsgBytes)
			if _, err := workerManager.ProducerSocket.Write(raw); err != nil {
				fmt.Printf("write producer error: %v\n", err)
			}
		}
		//重新生成transportId
		consumerTransportId := common_util.RandStringBytes(10)
		id := common_util.GneId()
		idStr := strconv.FormatInt(id, 10)
		consumeFinger := common_util.RandomStringWithHyphen(32)
		consumePassword := common_util.RandomStringWithHyphen(32)
		msg := idStr + ":router.createWebRtcTransportWithServer:" + roomId + ":{\"transportId\":\"" + consumerTransportId + "\",\"finger\":\"" + consumeFinger + "\",\"password\":\"" + consumePassword + "\",\"listenIps\":[{\"ip\":\"" + localIp + "\"}],\"port\":" + strconv.FormatInt(int64(udpPort), 10) + ",\"webRtcServerId\":\"001\",\"enableUdp\":true,\"enableTcp\":false,\"preferUdp\":false,\"preferTcp\":false,\"initialAvailableOutgoingBitrate\":1000000,\"enableSctp\":true,\"numSctpStreams\":{\"OS\":1024,\"MIS\":1024},\"maxSctpMessageSize\":262144,\"sctpSendBufferSize\":262144,\"isDataChannel\":true}"
		workerMsgBytes := []byte(msg + "\000")
		raw := common_util.Encode(workerMsgBytes)
		if _, err := workerManager.ProducerSocket.Write(raw); err != nil {
			return err
		}
		var consumerTransport = bean.ConsumerTransport{
			TransportId:    consumerTransportId,
			Ip:             localIp,
			Port:           udpPort,
			Fingerprint:    consumeFinger,
			FingerPassword: consumePassword,
			Consumers:      make(map[string]map[string]bean.Consumer),
		}
		roomUsers[index].Consumer = consumerTransport
		id = common_util.GneId()
		idStr = strconv.FormatInt(id, 10)
		msg = idStr + ":transport.enableTraceEvent:" + consumerTransportId + ":{\"types\":[\"bwe\"]}"
		workerMsgBytes = []byte(msg + "\000")
		raw = common_util.Encode(workerMsgBytes)
		if _, err := workerManager.ProducerSocket.Write(raw); err != nil {
			return err
		}
		id = common_util.GneId()
		idStr = strconv.FormatInt(id, 10)
		msg = idStr + ":transport.setMaxIncomingBitrate:" + consumerTransportId + ":{\"bitrate\":1500000}"
		workerMsgBytes = []byte(msg + "\000")
		raw = common_util.Encode(workerMsgBytes)
		if _, err := workerManager.ProducerSocket.Write(raw); err != nil {
			return err
		}
		//生成新的消费流
		for _, otherUser := range users {
			remoteConsumer := map[string]bean.Consumer{}
			remoteUserId := otherUser.UserId
			for producerId, producer := range otherUser.Producer.Producers {
				//传给worker的parameters
				transType := "simple"
				for _, encoding := range producer.RtpParameters.Encodings {
					if encoding.Rid != "" {
						transType = "simulcast"
					}
				}
				fmt.Printf("transType:%v\n", transType)
				if transType == "simple" {
					parameters := producer.RtpParameters
					consumableRtpEncodings := make([]bean.ConsumableRtpEncodings, 0)
					for i, ssrc := range producer.RtpMapping.Encodings {
						if i == 0 {
							consumableRtpEncodings = append(consumableRtpEncodings, bean.ConsumableRtpEncodings{
								Ssrc: ssrc.MappedSsrc,
							})
							break
						}
					}
					for i, _ := range parameters.Encodings {
						ssrc := common_util.GenerateSsrc()
						parameters.Encodings[i].Ssrc = ssrc
						rtx := parameters.Encodings[i].Rtx
						if rtx != nil && rtx["ssrc"] != 0 {
							rtx["ssrc"] = ssrc + 1
						}
						parameters.Encodings[i].Rtx = rtx
					}
					if producer.Kind == "audio" {
						parameters.Mid = "0"
						for i, _ := range parameters.Codecs {
							parameters.Codecs[i].PayloadType = 100
							parameters.Rtcp = bean.Rtcp{
								Cname:       "audio-" + producerId,
								ReducedSize: true,
								Mux:         true,
							}
						}
					} else {
						parameters.Mid = "1"
						for i, codecs := range producer.RtpParameters.Codecs {
							if codecs.Parameters["apt"] != nil {
								codecs.Parameters["apt"] = 101
							}
							if codecs.PayloadType == 96 {
								parameters.Codecs[i].PayloadType = 101
							}
							if codecs.PayloadType == 97 {
								parameters.Codecs[i].PayloadType = 102
							}
						}
						parameters.Rtcp = bean.Rtcp{
							Cname:       "video-" + producerId,
							ReducedSize: true,
							Mux:         true,
						}
					}
					//深度复制一份
					b, _ := json.Marshal(parameters)
					var rtpParameters bean.RtpParameters
					json.Unmarshal(b, &rtpParameters)
					typeStr := "simple"
					consumerId := common_util.RandStringBytes(10)
					consumer := bean.Consumer{
						ProducerId:             producerId,
						Type:                   typeStr,
						ConsumerId:             consumerId,
						Kind:                   producer.Kind,
						RtpParameters:          rtpParameters,
						ConsumableRtpEncodings: consumableRtpEncodings,
					}
					remoteConsumer[consumerId] = consumer
				} else if transType == "simulcast" {
					parameters := producer.RtpParameters
					consumableRtpEncodings := make([]bean.ConsumableRtpEncodings, 0)
					var maxBitrate = 0
					for i, encoding := range producer.RtpMapping.Encodings {
						encode := producer.RtpParameters.Encodings[i]
						maxBitrate = max(maxBitrate, encode.MaxBitrate)
						consumableRtpEncodings = append(consumableRtpEncodings, bean.ConsumableRtpEncodings{
							Active:                encode.Active,
							ScalabilityMode:       encode.ScalabilityMode,
							ScaleResolutionDownBy: encode.ScaleResolutionDownBy,
							MaxBitrate:            encode.MaxBitrate,
							Dtx:                   false,
							Ssrc:                  encoding.MappedSsrc,
						})
					}
					parameters.Encodings = make([]bean.Encoding, 0)
					ssrc := common_util.GenerateSsrc()
					rtx := map[string]uint32{}
					rtx["ssrc"] = ssrc + 1
					parameters.Encodings = append(parameters.Encodings, bean.Encoding{
						Ssrc:            ssrc,
						Rtx:             rtx,
						ScalabilityMode: "L3T3",
						MaxBitrate:      50000000,
					})
					parameters.Mid = "1"
					for i, codecs := range producer.RtpParameters.Codecs {
						if codecs.Parameters["apt"] != nil {
							codecs.Parameters["apt"] = 101
						}
						if codecs.PayloadType == 96 {
							parameters.Codecs[i].PayloadType = 101
						}
						if codecs.PayloadType == 97 {
							parameters.Codecs[i].PayloadType = 102
						}
						if i == 0 {
							producer.RtpParameters.Codecs[i].RtcpFeedback = make([]bean.RtcpFeedback, 0)
							producer.RtpParameters.Codecs[i].RtcpFeedback = append(producer.RtpParameters.Codecs[i].RtcpFeedback, bean.RtcpFeedback{
								Type:      "transport-cc",
								Parameter: "",
							})
							producer.RtpParameters.Codecs[i].RtcpFeedback = append(producer.RtpParameters.Codecs[i].RtcpFeedback, bean.RtcpFeedback{
								Type:      "ccm",
								Parameter: "fir",
							})
							producer.RtpParameters.Codecs[i].RtcpFeedback = append(producer.RtpParameters.Codecs[i].RtcpFeedback, bean.RtcpFeedback{
								Type:      "nack",
								Parameter: "",
							})
							producer.RtpParameters.Codecs[i].RtcpFeedback = append(producer.RtpParameters.Codecs[i].RtcpFeedback, bean.RtcpFeedback{
								Type:      "nack",
								Parameter: "fir",
							})
						}
					}
					parameters.Rtcp = bean.Rtcp{
						Cname:       "video-" + producerId,
						ReducedSize: true,
						Mux:         true,
					}
					parameters.HeaderExtensions = make([]bean.HeaderExtension, 0)
					parameters.HeaderExtensions = append(parameters.HeaderExtensions, bean.HeaderExtension{
						Uri:     "urn:ietf:params:rtp-hdrext:sdes:mid",
						Id:      1,
						Encrypt: false,
					})
					parameters.HeaderExtensions = append(parameters.HeaderExtensions, bean.HeaderExtension{
						Uri:     "http://www.webrtc.org/experiments/rtp-hdrext/abs-send-time",
						Id:      4,
						Encrypt: false,
					})
					parameters.HeaderExtensions = append(parameters.HeaderExtensions, bean.HeaderExtension{
						Uri:     "http://www.ietf.org/id/draft-holmer-rmcat-transport-wide-cc-extensions-01",
						Id:      5,
						Encrypt: false,
					})
					parameters.HeaderExtensions = append(parameters.HeaderExtensions, bean.HeaderExtension{
						Uri:     "urn:3gpp:video-orientation",
						Id:      11,
						Encrypt: false,
					})
					parameters.HeaderExtensions = append(parameters.HeaderExtensions, bean.HeaderExtension{
						Uri:     "urn:ietf:params:rtp-hdrext:toffset",
						Id:      12,
						Encrypt: false,
					})
					//深度复制一份
					b, _ := json.Marshal(parameters)
					var rtpParameters bean.RtpParameters
					json.Unmarshal(b, &rtpParameters)
					typeStr := "simulcast"
					consumerId := common_util.RandStringBytes(10)
					consumer := bean.Consumer{
						ProducerId:             producerId,
						Type:                   typeStr,
						ConsumerId:             consumerId,
						Kind:                   producer.Kind,
						RtpParameters:          rtpParameters,
						ConsumableRtpEncodings: consumableRtpEncodings,
					}
					remoteConsumer[consumerId] = consumer
				} else {
					fmt.Printf("err \n")
				}
			}
			roomUsers[index].Consumer.Consumers[remoteUserId] = remoteConsumer
		}
		temp, _ := room.UserIdUserMap.Load(notifyUserId)
		notifyUser = temp.(*bean.User)
		//组装 发送 offerSdp
		sdp := common_util.GenerateDownOfferSdp("sha-256", shaMap["sha-256"], notifyUser)
		var offerSdp = map[string]string{}
		offerSdp["sdp"] = sdp
		offerSdp["type"] = "offer"
		var offerSdpMap = make(map[string]interface{})
		offerSdpMap["type"] = "offer"
		offerSdpMap["offer"] = offerSdp
		offerSdpMapDetail, _ := json.Marshal(offerSdpMap)
		conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
		err := notifyUser.Conn.WriteMessage(websocket.TextMessage, offerSdpMapDetail)
		if err != nil {
			fmt.Printf("write offerSdp error: %v\n", err)
			notifyUser.Conn.WriteMessage(websocket.TextMessage, offerSdpMapDetail)
		}
	}
	return nil
}

func ClearUserInfo(conn *websocket.Conn) {
	cleanRoomId := ""
	roomMap.Range(func(roomId, value interface{}) bool {
		fmt.Printf("roomId:%v\n", roomId)
		cleanRoomId = roomId.(string)
		room := value.(*bean.Room)
		var total = 0
		room.UserIdUserMap.Range(func(userId, userValue any) bool {
			user := userValue.(*bean.User)
			if user.Conn.RemoteAddr() == conn.RemoteAddr() {
				roomId = user.RoomId
				producerTransportId := user.Producer.TransportId
				for _, media := range user.Producer.Producers {
					producerId := media.ProducerId
					id := common_util.GneId()
					idStr := strconv.FormatInt(id, 10)
					msg := idStr + ":transport.closeProducer:" + producerTransportId + ":{\"producerId\":\"" + producerId + "\"}"
					workerMsgBytes := []byte(msg + "\000")
					raw := common_util.Encode(workerMsgBytes)
					if _, err := workerManager.ProducerSocket.Write(raw); err != nil {
						fmt.Printf("write producer error: %v\n", err)
					}
				}
				//关闭transport
				id := common_util.GneId()
				idStr := strconv.FormatInt(id, 10)
				msg := idStr + ":router.closeTransport:" + cleanRoomId + ":{\"transportId\":\"" + producerTransportId + "\"}"
				workerMsgBytes := []byte(msg + "\000")
				raw := common_util.Encode(workerMsgBytes)
				if _, err := workerManager.ProducerSocket.Write(raw); err != nil {
					fmt.Printf("write producer error: %v\n", err)
				}
				consumerTransportId := user.Consumer.TransportId
				for _, media := range user.Consumer.Consumers {
					for _, consumer := range media {
						consumerId := consumer.ConsumerId
						id := common_util.GneId()
						idStr := strconv.FormatInt(id, 10)
						msg := idStr + ":transport.closeConsumer:" + consumerTransportId + ":{\"consumerId\":\"" + consumerId + "\"}"
						workerMsgBytes := []byte(msg + "\000")
						raw := common_util.Encode(workerMsgBytes)
						if _, err := workerManager.ProducerSocket.Write(raw); err != nil {
							fmt.Printf("write producer error: %v\n", err)
						}
					}
				}
				id = common_util.GneId()
				idStr = strconv.FormatInt(id, 10)
				msg = idStr + ":router.closeTransport:" + cleanRoomId + ":{\"transportId\":\"" + consumerTransportId + "\"}"
				workerMsgBytes = []byte(msg + "\000")
				raw = common_util.Encode(workerMsgBytes)
				if _, err := workerManager.ProducerSocket.Write(raw); err != nil {
					fmt.Printf("write producer error: %v\n", err)
				}
				room.UserIdUserMap.Delete(user.UserId)
			} else {
				total += 1
			}
			return true
		})
		if total == 0 {
			fmt.Printf("roomId: %v has non user in this room\n", roomId)
			id := common_util.GneId()
			idStr := strconv.FormatInt(id, 10)
			msg := idStr + ":worker.closeRouter:req-123:{\"routerId\":\"" + cleanRoomId + "\"}"
			workerMsgBytes := []byte(msg + "\000")
			raw := common_util.Encode(workerMsgBytes)
			workerManager.ProducerSocket.Write(raw)
			roomMap.Delete(cleanRoomId)
		}
		return true
	})
	//更新房间中其他用户下行订阅
	if cleanRoomId != "" {
		fmt.Printf("has user leave room %s and update DownOfferSdp \n", cleanRoomId)
		ProcessDownOfferSdp(cleanRoomId)
	}
}

func ProcessSignal(message []byte, conn *websocket.Conn) error {
	msgMap := map[string]string{}
	json.Unmarshal(message, &msgMap)
	s := msgMap["signal"]
	id := common_util.GneId()
	idStr := strconv.FormatInt(id, 10)
	msg := idStr + ":" + s
	workerMsgBytes := []byte(msg + "\000")
	raw := common_util.Encode(workerMsgBytes)
	if _, err := workerManager.ProducerSocket.Write(raw); err != nil {
		return err
	}
	return nil
}

// 测试级联
func ProcessPipe01(message []byte, conn *websocket.Conn) error {
	var offer bean.OfferReq
	err := json.Unmarshal(message, &offer)
	if err != nil {
		return err
	}
	var activeSpeakerObserverId string
	var audioLevelObserverId string
	roomId := offer.RoomId
	userId := offer.UserId
	fmt.Printf("offer:\n %v\n", offer.Offer.Sdp)
	value, ok := roomMap.Load(roomId)
	var room *bean.Room
	user := &bean.User{
		Conn:                   conn,
		UserId:                 userId,
		RoomId:                 roomId,
		WorkerIdTransportIdMap: make(map[int64]string), // 初始化新映射
	}
	//房间不存在
	if !ok {
		id := common_util.GneId()
		idStr := strconv.FormatInt(id, 10)
		msg := idStr + ":worker.createRouter:req-123:{\"routerId\":\"" + roomId + "\"}"
		workerMsgBytes := []byte(msg + "\000")
		raw := common_util.Encode(workerMsgBytes)
		if _, err := workerManager.ProducerSocket.Write(raw); err != nil {
			return err
		}
		audioLevelObserverId = common_util.RandStringBytes(10)
		id = common_util.GneId()
		idStr = strconv.FormatInt(id, 10)
		msg = idStr + ":router.createAudioLevelObserver:" + roomId + ":{\"rtpObserverId\":\"" + audioLevelObserverId + "\",\"maxEntries\":1,\"threshold\":-80,\"interval\":800}"
		workerMsgBytes = []byte(msg + "\000")
		raw = common_util.Encode(workerMsgBytes)
		if _, err = workerManager.ProducerSocket.Write(raw); err != nil {
			return err
		}
		log.Printf("Sending message to worker: %s", string(raw))
		activeSpeakerObserverId = common_util.RandStringBytes(10)
		id = common_util.GneId()
		idStr = strconv.FormatInt(id, 10)
		msg = idStr + ":router.createActiveSpeakerObserver:" + roomId + ":{\"rtpObserverId\":\"" + activeSpeakerObserverId + "\",\"interval\":300}"
		workerMsgBytes = []byte(msg + "\000")
		raw = common_util.Encode(workerMsgBytes)
		if _, err = workerManager.ProducerSocket.Write(raw); err != nil {
			return err
		}
		userIdUserMap := sync.Map{}
		userIdUserMap.Store(userId, user)
		room = &bean.Room{
			ActiveSpeakerObserverId: audioLevelObserverId,
			AudioLevelObserverId:    activeSpeakerObserverId,
			UserIdUserMap:           userIdUserMap,
		}
		roomMap.Store(roomId, room)

		roomId_pipe02 := "lsk01pipe02"
		id = common_util.GneId()
		idStr = strconv.FormatInt(id, 10)
		msg = idStr + ":worker.createRouter:req-123:{\"routerId\":\"" + roomId_pipe02 + "\"}"
		workerMsgBytes = []byte(msg + "\000")
		raw = common_util.Encode(workerMsgBytes)
		if _, err := workerManager.ProducerSocket.Write(raw); err != nil {
			return err
		}

	} else {
		//房间存在
		room = value.(*bean.Room) // 断言为指针类型
		fmt.Printf("roomId:%v alread exist", roomId)
		activeSpeakerObserverId = room.ActiveSpeakerObserverId
		audioLevelObserverId = room.AudioLevelObserverId
		room.UserIdUserMap.Store(userId, user)
	}
	user.Mu.Lock()
	defer user.Mu.Unlock()
	fmt.Printf("roomId:%v userId:%v\n", roomId, userId)
	producerTransportId := common_util.RandStringBytes(10)
	id := common_util.GneId()
	idStr := strconv.FormatInt(id, 10)
	user.WorkerIdTransportIdMap[id] = producerTransportId
	producerFinger := common_util.RandomStringWithHyphen(32)
	producerPassword := common_util.RandomStringWithHyphen(32)
	msg := idStr + ":router.createWebRtcTransportWithServer:" + roomId + ":{\"transportId\":\"" + producerTransportId + "\",\"finger\":\"" + producerFinger + "\",\"password\":\"" + producerPassword + "\",\"listenIps\":[{\"ip\":\"" + localIp + "\"}],\"port\":" + strconv.FormatInt(int64(udpPort), 10) + ",\"webRtcServerId\":\"001\",\"enableUdp\":true,\"enableTcp\":false,\"preferUdp\":false,\"preferTcp\":false,\"initialAvailableOutgoingBitrate\":1000000,\"enableSctp\":true,\"numSctpStreams\":{\"OS\":1024,\"MIS\":1024},\"maxSctpMessageSize\":262144,\"sctpSendBufferSize\":262144,\"isDataChannel\":true}"
	workerMsgBytes := []byte(msg + "\000")
	raw := common_util.Encode(workerMsgBytes)
	if _, err = workerManager.ProducerSocket.Write(raw); err != nil {
		return err
	}
	producerTransport := bean.ProducerTransport{
		TransportId:    producerTransportId,
		Ip:             localIp,
		Port:           udpPort,
		Fingerprint:    producerFinger,
		FingerPassword: producerPassword,
		Producers:      make(map[string]bean.MediaParameters),
	}
	user.Producer = producerTransport

	//创建第一个pipe
	id = common_util.GneId()
	idStr = strconv.FormatInt(id, 10)
	msg = idStr + ":router.createPipeTransport:lsk01pipe01:{\"transportId\":\"pipe-01\",\"listenIp\":{\"ip\":\"" + localIp + "\"},\"port\":46560,\"enableSctp\":false,\"numSctpStreams\":{\"OS\":1024,\"MIS\":1024},\"maxSctpMessageSize\":268435456,\"sctpSendBufferSize\":268435456,\"isDataChannel\":false,\"enableRtx\":false,\"enableSrtp\":false}"
	workerMsgBytes = []byte(msg + "\000")
	raw = common_util.Encode(workerMsgBytes)
	if _, err = workerManager.ProducerSocket.Write(raw); err != nil {
		return err
	}
	time.Sleep(20 * time.Millisecond)
	id = common_util.GneId()
	idStr = strconv.FormatInt(id, 10)
	msg = idStr + ":router.createPipeTransport:lsk01pipe02:{\"transportId\":\"pipe-02\",\"listenIp\":{\"ip\":\"" + localIp + "\"},\"port\":46561,\"enableSctp\":false,\"numSctpStreams\":{\"OS\":1024,\"MIS\":1024},\"maxSctpMessageSize\":268435456,\"sctpSendBufferSize\":268435456,\"isDataChannel\":false,\"enableRtx\":false,\"enableSrtp\":false}"
	workerMsgBytes = []byte(msg + "\000")
	raw = common_util.Encode(workerMsgBytes)
	if _, err = workerManager.ProducerSocket.Write(raw); err != nil {
		return err
	}
	time.Sleep(20 * time.Millisecond)
	//相互连接
	id = common_util.GneId()
	idStr = strconv.FormatInt(id, 10)
	msg = idStr + ":transport.connect:pipe-01:{\"ip\":\"" + localIp + "\",\"port\":46561}"
	workerMsgBytes = []byte(msg + "\000")
	raw = common_util.Encode(workerMsgBytes)
	if _, err = workerManager.ProducerSocket.Write(raw); err != nil {
		return err
	}
	time.Sleep(20 * time.Millisecond)
	id = common_util.GneId()
	idStr = strconv.FormatInt(id, 10)
	msg = idStr + ":transport.connect:pipe-02:{\"ip\":\"" + localIp + "\",\"port\":46560}"
	workerMsgBytes = []byte(msg + "\000")
	raw = common_util.Encode(workerMsgBytes)
	if _, err = workerManager.ProducerSocket.Write(raw); err != nil {
		return err
	}
	time.Sleep(20 * time.Millisecond)
	id = common_util.GneId()
	idStr = strconv.FormatInt(id, 10)
	msg = idStr + ":transport.enableTraceEvent:" + producerTransportId + ":{\"types\":[\"bwe\"]}"
	workerMsgBytes = []byte(msg + "\000")
	raw = common_util.Encode(workerMsgBytes)
	if _, err = workerManager.ProducerSocket.Write(raw); err != nil {
		return err
	}
	id = common_util.GneId()
	idStr = strconv.FormatInt(id, 10)
	msg = idStr + ":transport.setMaxIncomingBitrate:" + producerTransportId + ":{\"bitrate\":1500000}"
	workerMsgBytes = []byte(msg + "\000")
	raw = common_util.Encode(workerMsgBytes)
	if _, err = workerManager.ProducerSocket.Write(raw); err != nil {
		return err
	}
	name, content := common_util.GetFingerNameContent(offer.Offer.Sdp)

	id = common_util.GneId()
	idStr = strconv.FormatInt(id, 10)
	msg = idStr + ":transport.connect:" + producerTransportId + ":{\"dtlsParameters\":{\"fingerprints\": [{\"algorithm\":\"" + name + "\",\"value\":\"" + content + "\"}],\"role\":\"auto\"}}"
	workerMsgBytes = []byte(msg + "\000")
	raw = common_util.Encode(workerMsgBytes)
	if _, err = workerManager.ProducerSocket.Write(raw); err != nil {
		return err
	}
	audioParams, videoParams, err := common_util.ParseUpOfferSDP(offer.Offer.Sdp)
	if err != nil {
		return err
	}
	audioProduceId := common_util.RandStringBytes(10)
	videoProduceId := common_util.RandStringBytes(10)
	//audioProduceId := "produceaudio"
	//videoProduceId := "producevideo"
	audioParams.ProducerId = audioProduceId
	videoParams.ProducerId = videoProduceId
	producerTransport.Producers[audioProduceId] = audioParams
	producerTransport.Producers[videoProduceId] = videoParams
	audio, err := json.Marshal(audioParams)
	if err != nil {
		return err
	}
	video, err := json.Marshal(videoParams)
	if err != nil {
		return err
	}
	id = common_util.GneId()
	idStr = strconv.FormatInt(id, 10)
	msg = idStr + ":transport.produce:" + producerTransportId + ":" + string(audio)
	workerMsgBytes = []byte(msg + "\000")
	raw = common_util.Encode(workerMsgBytes)
	if _, err = workerManager.ProducerSocket.Write(raw); err != nil {
		return err
	}

	//pipe01 consume audio
	pipe_ssrc_auido := common_util.GenerateSsrc()
	id = common_util.GneId()
	idStr = strconv.FormatInt(id, 10)
	msg = idStr + ":transport.consume:pipe-01:" + "{\"consumerId\":\"47728e42-ab7b-4721-9005-985d04ab5a2b\",\"producerId\":\"" + audioProduceId + "\",\"kind\":\"audio\",\"rtpParameters\":{\"codecs\":[{\"mimeType\":\"audio/opus\",\"payloadType\":100,\"clockRate\":48000,\"channels\":2,\"parameters\":{\"minptime\":10,\"useinbandfec\":1},\"rtcpFeedback\":[]}],\"headerExtensions\":[{\"uri\":\"urn:ietf:params:rtp-hdrext:ssrc-audio-level\",\"id\":10,\"encrypt\":false,\"parameters\":{}},{\"uri\":\"http://www.webrtc.org/experiments/rtp-hdrext/abs-capture-time\",\"id\":13,\"encrypt\":false,\"parameters\":{}}],\"encodings\":[{\"ssrc\":" + strconv.FormatInt(int64(pipe_ssrc_auido), 10) + ",\"dtx\":false}],\"rtcp\":{\"cname\":\"eks80YQXECwcuNgX\",\"reducedSize\":true,\"mux\":true}},\"type\":\"pipe\",\"consumableRtpEncodings\":[{\"ssrc\":" + strconv.FormatInt(int64(audioParams.RtpMapping.Encodings[0].MappedSsrc), 10) + ",\"dtx\":false}]}"
	workerMsgBytes = []byte(msg + "\000")
	raw = common_util.Encode(workerMsgBytes)
	if _, err = workerManager.ProducerSocket.Write(raw); err != nil {
		return err
	}
	time.Sleep(20 * time.Millisecond)
	id = common_util.GneId()
	idStr = strconv.FormatInt(id, 10)
	msg = idStr + ":rtpObserver.addProducer:" + audioLevelObserverId + ":{\"producerId\":\"" + audioProduceId + "\"}"
	workerMsgBytes = []byte(msg + "\000")
	raw = common_util.Encode(workerMsgBytes)
	if _, err = workerManager.ProducerSocket.Write(raw); err != nil {
		return err
	}
	time.Sleep(20 * time.Millisecond)
	id = common_util.GneId()
	idStr = strconv.FormatInt(id, 10)
	msg = idStr + ":transport.produce:" + producerTransportId + ":" + string(video)
	workerMsgBytes = []byte(msg + "\000")
	raw = common_util.Encode(workerMsgBytes)
	if _, err = workerManager.ProducerSocket.Write(raw); err != nil {
		return err
	}
	//pipe01 consume video
	pipe_ssrc_video := common_util.GenerateSsrc()
	id = common_util.GneId()
	idStr = strconv.FormatInt(id, 10)
	msg = idStr + ":transport.consume:pipe-01:" + "{\"consumerId\":\"bf3d34db-8569-44c7-89d1-8a4b9ae5e26d\",\"producerId\":\"" + videoProduceId + "\",\"kind\":\"video\",\"rtpParameters\":{\"codecs\":[{\"mimeType\":\"video/VP8\",\"payloadType\":101,\"clockRate\":90000,\"parameters\":{},\"rtcpFeedback\":[{\"type\":\"nack\",\"parameter\":\"pli\"},{\"type\":\"ccm\",\"parameter\":\"fir\"}]}],\"headerExtensions\":[{\"uri\":\"http://tools.ietf.org/html/draft-ietf-avtext-framemarking-07\",\"id\":6,\"encrypt\":false,\"parameters\":{}},{\"uri\":\"urn:ietf:params:rtp-hdrext:framemarking\",\"id\":7,\"encrypt\":false,\"parameters\":{}},{\"uri\":\"urn:3gpp:video-orientation\",\"id\":11,\"encrypt\":false,\"parameters\":{}},{\"uri\":\"urn:ietf:params:rtp-hdrext:toffset\",\"id\":12,\"encrypt\":false,\"parameters\":{}},{\"uri\":\"http://www.webrtc.org/experiments/rtp-hdrext/abs-capture-time\",\"id\":13,\"encrypt\":false,\"parameters\":{}}],\"encodings\":[{\"ssrc\":" + strconv.FormatInt(int64(pipe_ssrc_video), 10) + ",\"dtx\":false}],\"rtcp\":{\"cname\":\"eks80YQXECwcuNgX\",\"reducedSize\":true,\"mux\":true}},\"type\":\"pipe\",\"consumableRtpEncodings\":[{\"ssrc\":" + strconv.FormatInt(int64(videoParams.RtpMapping.Encodings[0].MappedSsrc), 10) + ",\"dtx\":false}]}"
	workerMsgBytes = []byte(msg + "\000")
	raw = common_util.Encode(workerMsgBytes)
	if _, err = workerManager.ProducerSocket.Write(raw); err != nil {
		return err
	}
	time.Sleep(20 * time.Millisecond)
	//pipe02 produce audio
	pipe02_ssrc_audio = common_util.GenerateSsrc()
	id = common_util.GneId()
	idStr = strconv.FormatInt(id, 10)
	msg = idStr + ":transport.produce:pipe-02:" + "{\"producerId\":\"produceaudio\",\"kind\":\"audio\",\"rtpParameters\":{\"codecs\":[{\"mimeType\":\"audio/opus\",\"payloadType\":100,\"clockRate\":48000,\"channels\":2,\"parameters\":{\"minptime\":10,\"useinbandfec\":1},\"rtcpFeedback\":[]}],\"headerExtensions\":[{\"uri\":\"urn:ietf:params:rtp-hdrext:ssrc-audio-level\",\"id\":10,\"encrypt\":false,\"parameters\":{}},{\"uri\":\"http://www.webrtc.org/experiments/rtp-hdrext/abs-capture-time\",\"id\":13,\"encrypt\":false,\"parameters\":{}}],\"encodings\":[{\"ssrc\":" + strconv.FormatInt(int64(pipe_ssrc_auido), 10) + ",\"dtx\":false}],\"rtcp\":{\"cname\":\"eks80YQXECwcuNgX\",\"reducedSize\":true,\"mux\":true}},\"rtpMapping\":{\"codecs\":[{\"payloadType\":100,\"mappedPayloadType\":100}],\"encodings\":[{\"mappedSsrc\":" + strconv.FormatInt(int64(pipe02_ssrc_audio), 10) + ",\"ssrc\":" + strconv.FormatInt(int64(pipe_ssrc_auido), 10) + "}]},\"paused\":false}"
	workerMsgBytes = []byte(msg + "\000")
	raw = common_util.Encode(workerMsgBytes)
	if _, err = workerManager.ProducerSocket.Write(raw); err != nil {
		return err
	}
	time.Sleep(20 * time.Millisecond)
	//pipe02 produce video
	pipe02_ssrc_video = common_util.GenerateSsrc()
	id = common_util.GneId()
	idStr = strconv.FormatInt(id, 10)
	msg = idStr + ":transport.produce:pipe-02:" + "{\"producerId\":\"producevideo\",\"kind\":\"video\",\"rtpParameters\":{\"codecs\":[{\"mimeType\":\"video/VP8\",\"payloadType\":101,\"clockRate\":90000,\"parameters\":{},\"rtcpFeedback\":[{\"type\":\"nack\",\"parameter\":\"pli\"},{\"type\":\"ccm\",\"parameter\":\"fir\"}]}],\"headerExtensions\":[{\"uri\":\"http://tools.ietf.org/html/draft-ietf-avtext-framemarking-07\",\"id\":6,\"encrypt\":false,\"parameters\":{}},{\"uri\":\"urn:ietf:params:rtp-hdrext:framemarking\",\"id\":7,\"encrypt\":false,\"parameters\":{}},{\"uri\":\"urn:3gpp:video-orientation\",\"id\":11,\"encrypt\":false,\"parameters\":{}},{\"uri\":\"urn:ietf:params:rtp-hdrext:toffset\",\"id\":12,\"encrypt\":false,\"parameters\":{}},{\"uri\":\"http://www.webrtc.org/experiments/rtp-hdrext/abs-capture-time\",\"id\":13,\"encrypt\":false,\"parameters\":{}}],\"encodings\":[{\"ssrc\":" + strconv.FormatInt(int64(pipe_ssrc_video), 10) + ",\"dtx\":false}],\"rtcp\":{\"cname\":\"eks80YQXECwcuNgX\",\"reducedSize\":true,\"mux\":true}},\"rtpMapping\":{\"codecs\":[{\"payloadType\":101,\"mappedPayloadType\":101}],\"encodings\":[{\"mappedSsrc\":" + strconv.FormatInt(int64(pipe02_ssrc_video), 10) + ",\"ssrc\":" + strconv.FormatInt(int64(pipe_ssrc_video), 10) + "}]},\"paused\":false}"
	workerMsgBytes = []byte(msg + "\000")
	raw = common_util.Encode(workerMsgBytes)
	if _, err = workerManager.ProducerSocket.Write(raw); err != nil {
		return err
	}

	time.Sleep(20 * time.Millisecond)
	//上行answer
	ProcessUpAnswerSdp(user, offer.Offer.Sdp)
	//下行订阅
	time.Sleep(20 * time.Millisecond)
	ProcessDownOfferSdp(roomId)
	return nil
}

// 测试级联
func ProcessPipe02(message []byte, conn *websocket.Conn) error {
	var offer bean.OfferReq
	err := json.Unmarshal(message, &offer)
	if err != nil {
		return err
	}
	var activeSpeakerObserverId string
	var audioLevelObserverId string
	roomId := "lsk01pipe01"
	userId := offer.UserId
	fmt.Printf("offer:\n %v\n", offer.Offer.Sdp)
	value, ok := roomMap.Load(roomId)
	var room *bean.Room
	user := &bean.User{
		Conn:                   conn,
		UserId:                 userId,
		RoomId:                 roomId,
		WorkerIdTransportIdMap: make(map[int64]string), // 初始化新映射
	}
	//房间不存在
	if !ok {
		id := common_util.GneId()
		idStr := strconv.FormatInt(id, 10)
		msg := idStr + ":worker.createRouter:req-123:{\"routerId\":\"" + roomId + "\"}"
		workerMsgBytes := []byte(msg + "\000")
		raw := common_util.Encode(workerMsgBytes)
		if _, err := workerManager.ProducerSocket.Write(raw); err != nil {
			return err
		}
		audioLevelObserverId = common_util.RandStringBytes(10)
		id = common_util.GneId()
		idStr = strconv.FormatInt(id, 10)
		msg = idStr + ":router.createAudioLevelObserver:" + roomId + ":{\"rtpObserverId\":\"" + audioLevelObserverId + "\",\"maxEntries\":1,\"threshold\":-80,\"interval\":800}"
		workerMsgBytes = []byte(msg + "\000")
		raw = common_util.Encode(workerMsgBytes)
		if _, err = workerManager.ProducerSocket.Write(raw); err != nil {
			return err
		}
		log.Printf("Sending message to worker: %s", string(raw))
		activeSpeakerObserverId = common_util.RandStringBytes(10)
		id = common_util.GneId()
		idStr = strconv.FormatInt(id, 10)
		msg = idStr + ":router.createActiveSpeakerObserver:" + roomId + ":{\"rtpObserverId\":\"" + activeSpeakerObserverId + "\",\"interval\":300}"
		workerMsgBytes = []byte(msg + "\000")
		raw = common_util.Encode(workerMsgBytes)
		if _, err = workerManager.ProducerSocket.Write(raw); err != nil {
			return err
		}
		userIdUserMap := sync.Map{}
		userIdUserMap.Store(userId, user)
		room = &bean.Room{
			ActiveSpeakerObserverId: audioLevelObserverId,
			AudioLevelObserverId:    activeSpeakerObserverId,
			UserIdUserMap:           userIdUserMap,
		}
		roomMap.Store(roomId, room)
	} else {
		//房间存在
		room = value.(*bean.Room) // 断言为指针类型
		fmt.Printf("roomId:%v alread exist", roomId)
		activeSpeakerObserverId = room.ActiveSpeakerObserverId
		audioLevelObserverId = room.AudioLevelObserverId
		room.UserIdUserMap.Store(userId, user)
	}

	user.Mu.Lock()
	defer user.Mu.Unlock()
	fmt.Printf("roomId:%v userId:%v\n", roomId, userId)
	producerTransportId := common_util.RandStringBytes(10)
	id := common_util.GneId()
	idStr := strconv.FormatInt(id, 10)
	user.WorkerIdTransportIdMap[id] = producerTransportId
	producerFinger := common_util.RandomStringWithHyphen(32)
	producerPassword := common_util.RandomStringWithHyphen(32)
	msg := idStr + ":router.createWebRtcTransportWithServer:lsk01pipe02:{\"transportId\":\"" + producerTransportId + "\",\"finger\":\"" + producerFinger + "\",\"password\":\"" + producerPassword + "\",\"listenIps\":[{\"ip\":\"" + localIp + "\"}],\"port\":" + strconv.FormatInt(int64(udpPort), 10) + ",\"webRtcServerId\":\"001\",\"enableUdp\":true,\"enableTcp\":false,\"preferUdp\":false,\"preferTcp\":false,\"initialAvailableOutgoingBitrate\":1000000,\"enableSctp\":true,\"numSctpStreams\":{\"OS\":1024,\"MIS\":1024},\"maxSctpMessageSize\":262144,\"sctpSendBufferSize\":262144,\"isDataChannel\":true}"
	workerMsgBytes := []byte(msg + "\000")
	raw := common_util.Encode(workerMsgBytes)
	if _, err = workerManager.ProducerSocket.Write(raw); err != nil {
		return err
	}
	producerTransport := bean.ProducerTransport{
		TransportId:    producerTransportId,
		Ip:             localIp,
		Port:           udpPort,
		Fingerprint:    producerFinger,
		FingerPassword: producerPassword,
		Producers:      make(map[string]bean.MediaParameters),
	}
	user.Producer = producerTransport

	id = common_util.GneId()
	idStr = strconv.FormatInt(id, 10)
	msg = idStr + ":transport.enableTraceEvent:" + producerTransportId + ":{\"types\":[\"bwe\"]}"
	workerMsgBytes = []byte(msg + "\000")
	raw = common_util.Encode(workerMsgBytes)
	if _, err = workerManager.ProducerSocket.Write(raw); err != nil {
		return err
	}
	id = common_util.GneId()
	idStr = strconv.FormatInt(id, 10)
	msg = idStr + ":transport.setMaxIncomingBitrate:" + producerTransportId + ":{\"bitrate\":1500000}"
	workerMsgBytes = []byte(msg + "\000")
	raw = common_util.Encode(workerMsgBytes)
	if _, err = workerManager.ProducerSocket.Write(raw); err != nil {
		return err
	}
	name, content := common_util.GetFingerNameContent(offer.Offer.Sdp)

	id = common_util.GneId()
	idStr = strconv.FormatInt(id, 10)
	msg = idStr + ":transport.connect:" + producerTransportId + ":{\"dtlsParameters\":{\"fingerprints\": [{\"algorithm\":\"" + name + "\",\"value\":\"" + content + "\"}],\"role\":\"auto\"}}"
	workerMsgBytes = []byte(msg + "\000")
	raw = common_util.Encode(workerMsgBytes)
	if _, err = workerManager.ProducerSocket.Write(raw); err != nil {
		return err
	}
	audioParams, videoParams, err := common_util.ParseUpOfferSDP(offer.Offer.Sdp)
	if err != nil {
		return err
	}
	audioProduceId := common_util.RandStringBytes(10)
	videoProduceId := common_util.RandStringBytes(10)
	audioParams.ProducerId = audioProduceId
	videoParams.ProducerId = videoProduceId
	producerTransport.Producers[audioProduceId] = audioParams
	producerTransport.Producers[videoProduceId] = videoParams
	audio, err := json.Marshal(audioParams)
	if err != nil {
		return err
	}
	video, err := json.Marshal(videoParams)
	if err != nil {
		return err
	}
	id = common_util.GneId()
	idStr = strconv.FormatInt(id, 10)
	msg = idStr + ":transport.produce:" + producerTransportId + ":" + string(audio)
	workerMsgBytes = []byte(msg + "\000")
	raw = common_util.Encode(workerMsgBytes)
	if _, err = workerManager.ProducerSocket.Write(raw); err != nil {
		return err
	}

	id = common_util.GneId()
	idStr = strconv.FormatInt(id, 10)
	msg = idStr + ":transport.produce:" + producerTransportId + ":" + string(video)
	workerMsgBytes = []byte(msg + "\000")
	raw = common_util.Encode(workerMsgBytes)
	if _, err = workerManager.ProducerSocket.Write(raw); err != nil {
		return err
	}

	//pipe2 produc
	//上行answer
	ProcessUpAnswerSdp(user, offer.Offer.Sdp)
	//下行订阅
	time.Sleep(20 * time.Millisecond)
	ProcessPipeDownOfferSdp(roomId, user, pipe02_ssrc_audio, pipe02_ssrc_video)

	return nil
}

func ProcessPipeDownOfferSdp(roomId string, user *bean.User, audio_ssrc, video_ssrc uint32) error {
	roomValue, exit := roomMap.Load(roomId)
	if !exit {
		return fmt.Errorf("roomId %s not exist", roomId)
	}
	room := roomValue.(*bean.Room)
	roomUsers := make([]*bean.User, 0)
	room.UserIdUserMap.Range(func(key, value any) bool {
		user := value.(*bean.User)
		roomUsers = append(roomUsers, user)
		return true
	})
	if len(roomUsers) <= 1 {
		return nil
	}
	consumerTransportId := common_util.RandStringBytes(10)
	id := common_util.GneId()
	idStr := strconv.FormatInt(id, 10)
	consumeFinger := common_util.RandomStringWithHyphen(32)
	consumePassword := common_util.RandomStringWithHyphen(32)
	msg := idStr + ":router.createWebRtcTransportWithServer:lsk01pipe02:{\"transportId\":\"" + consumerTransportId + "\",\"finger\":\"" + consumeFinger + "\",\"password\":\"" + consumePassword + "\",\"listenIps\":[{\"ip\":\"" + localIp + "\"}],\"port\":" + strconv.FormatInt(int64(udpPort), 10) + ",\"webRtcServerId\":\"001\",\"enableUdp\":true,\"enableTcp\":false,\"preferUdp\":false,\"preferTcp\":false,\"initialAvailableOutgoingBitrate\":1000000,\"enableSctp\":true,\"numSctpStreams\":{\"OS\":1024,\"MIS\":1024},\"maxSctpMessageSize\":262144,\"sctpSendBufferSize\":262144,\"isDataChannel\":true}"
	workerMsgBytes := []byte(msg + "\000")
	raw := common_util.Encode(workerMsgBytes)
	if _, err := workerManager.ProducerSocket.Write(raw); err != nil {
		return err
	}

	id = common_util.GneId()
	idStr = strconv.FormatInt(id, 10)
	msg = idStr + ":transport.enableTraceEvent:" + consumerTransportId + ":{\"types\":[\"bwe\"]}"
	workerMsgBytes = []byte(msg + "\000")
	raw = common_util.Encode(workerMsgBytes)
	if _, err := workerManager.ProducerSocket.Write(raw); err != nil {
		return err
	}
	id = common_util.GneId()
	idStr = strconv.FormatInt(id, 10)
	msg = idStr + ":transport.setMaxIncomingBitrate:" + consumerTransportId + ":{\"bitrate\":1500000}"
	workerMsgBytes = []byte(msg + "\000")
	raw = common_util.Encode(workerMsgBytes)
	if _, err := workerManager.ProducerSocket.Write(raw); err != nil {
		return err
	}

	var consumerTransport = bean.ConsumerTransport{
		TransportId:    consumerTransportId,
		Ip:             localIp,
		Port:           udpPort,
		Fingerprint:    consumeFinger,
		FingerPassword: consumePassword,
		Consumers:      make(map[string]map[string]bean.Consumer),
	}
	user.Consumer = consumerTransport

	//直接生成对应的consume
	id = common_util.GneId()
	idStr = strconv.FormatInt(id, 10)
	audio := common_util.GenerateSsrc()
	msg = idStr + ":transport.consume:" + consumerTransportId + ":" + "{\"consumerId\":\"2187aa48-996f-4de8-8798-b7d00c56c227\",\"producerId\":\"produceaudio\",\"kind\":\"audio\",\"rtpParameters\":{\"codecs\":[{\"mimeType\":\"audio/opus\",\"payloadType\":100,\"clockRate\":48000,\"channels\":2,\"parameters\":{\"minptime\":10,\"useinbandfec\":1},\"rtcpFeedback\":[]}],\"headerExtensions\":[{\"uri\":\"urn:ietf:params:rtp-hdrext:sdes:mid\",\"id\":1,\"encrypt\":false,\"parameters\":{}},{\"uri\":\"http://www.webrtc.org/experiments/rtp-hdrext/abs-send-time\",\"id\":4,\"encrypt\":false,\"parameters\":{}},{\"uri\":\"urn:ietf:params:rtp-hdrext:ssrc-audio-level\",\"id\":10,\"encrypt\":false,\"parameters\":{}}],\"encodings\":[{\"ssrc\":" + strconv.FormatInt(int64(audio), 10) + "}],\"rtcp\":{\"cname\":\"eks80YQXECwcuNgX\",\"reducedSize\":true,\"mux\":true},\"mid\":\"0\"},\"type\":\"simple\",\"consumableRtpEncodings\":[{\"ssrc\":" + strconv.FormatInt(int64(audio_ssrc), 10) + ",\"dtx\":false}],\"paused\":true,\"ignoreDtx\":false}"
	workerMsgBytes = []byte(msg + "\000")
	raw = common_util.Encode(workerMsgBytes)
	if _, err := workerManager.ProducerSocket.Write(raw); err != nil {
		return err
	}
	id = common_util.GneId()
	idStr = strconv.FormatInt(id, 10)
	msg = idStr + ":consumer.resume:2187aa48-996f-4de8-8798-b7d00c56c227:{}"
	workerMsgBytes = []byte(msg + "\000")
	raw = common_util.Encode(workerMsgBytes)
	if _, err := workerManager.ProducerSocket.Write(raw); err != nil {
		return err
	}
	id = common_util.GneId()
	idStr = strconv.FormatInt(id, 10)
	video := common_util.GenerateSsrc()
	msg = idStr + ":transport.consume:" + consumerTransportId + ":" + "{\"consumerId\":\"62652d98-3ee5-4c21-8fa4-34baa14b35d7\",\"producerId\":\"producevideo\",\"kind\":\"video\",\"rtpParameters\":{\"codecs\":[{\"mimeType\":\"video/VP8\",\"payloadType\":101,\"clockRate\":90000,\"parameters\":{},\"rtcpFeedback\":[{\"type\":\"transport-cc\",\"parameter\":\"\"},{\"type\":\"ccm\",\"parameter\":\"fir\"},{\"type\":\"nack\",\"parameter\":\"\"},{\"type\":\"nack\",\"parameter\":\"pli\"}]},{\"mimeType\":\"video/rtx\",\"payloadType\":102,\"clockRate\":90000,\"parameters\":{\"apt\":101},\"rtcpFeedback\":[]}],\"headerExtensions\":[{\"uri\":\"urn:ietf:params:rtp-hdrext:sdes:mid\",\"id\":1,\"encrypt\":false,\"parameters\":{}},{\"uri\":\"http://www.webrtc.org/experiments/rtp-hdrext/abs-send-time\",\"id\":4,\"encrypt\":false,\"parameters\":{}},{\"uri\":\"http://www.ietf.org/id/draft-holmer-rmcat-transport-wide-cc-extensions-01\",\"id\":5,\"encrypt\":false,\"parameters\":{}},{\"uri\":\"urn:3gpp:video-orientation\",\"id\":11,\"encrypt\":false,\"parameters\":{}},{\"uri\":\"urn:ietf:params:rtp-hdrext:toffset\",\"id\":12,\"encrypt\":false,\"parameters\":{}}],\"encodings\":[{\"ssrc\":" + strconv.FormatInt(int64(video), 10) + ",\"rtx\":{\"ssrc\":" + strconv.FormatInt(int64(video+1), 10) + "}}],\"rtcp\":{\"cname\":\"eks80YQXECwcuNgX\",\"reducedSize\":true,\"mux\":true},\"mid\":\"1\"},\"type\":\"simple\",\"consumableRtpEncodings\":[{\"ssrc\":" + strconv.FormatInt(int64(video_ssrc), 10) + ",\"dtx\":false}],\"paused\":true,\"ignoreDtx\":false}"
	workerMsgBytes = []byte(msg + "\000")
	raw = common_util.Encode(workerMsgBytes)
	if _, err := workerManager.ProducerSocket.Write(raw); err != nil {
		return err
	}
	id = common_util.GneId()
	idStr = strconv.FormatInt(id, 10)
	msg = idStr + ":consumer.resume:62652d98-3ee5-4c21-8fa4-34baa14b35d7:{}"
	workerMsgBytes = []byte(msg + "\000")
	raw = common_util.Encode(workerMsgBytes)
	if _, err := workerManager.ProducerSocket.Write(raw); err != nil {
		return err
	}
	//生成下行offer
	sdp := common_util.GenerateDownOfferSdpPipe("sha-256", shaMap["sha-256"], user, audio, video)
	var offerSdp = map[string]string{}
	offerSdp["sdp"] = sdp
	offerSdp["type"] = "offer"
	var offerSdpMap = make(map[string]interface{})
	offerSdpMap["type"] = "offer"
	offerSdpMap["offer"] = offerSdp
	offerSdpMapDetail, _ := json.Marshal(offerSdpMap)
	fmt.Printf("send downoffer:%v\n", offerSdpMap)
	user.Conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	err := user.Conn.WriteMessage(websocket.TextMessage, offerSdpMapDetail)
	if err != nil {
		fmt.Printf("write offerSdp error: %v\n", err)
		user.Conn.WriteMessage(websocket.TextMessage, offerSdpMapDetail)
	}
	return nil
}

func ProcessAnswerPipe02(message []byte, conn *websocket.Conn) error {
	var answer bean.AnswerReq
	err := json.Unmarshal(message, &answer)
	if err != nil {
		fmt.Printf("[ProcessDownAnswerSdp] json.Unmarshal err:%v\n", err)
		return err
	}
	userId := answer.UserId
	roomId := "lsk01pipe01"
	roomValue, exist := roomMap.Load(roomId)
	if !exist {
		return fmt.Errorf("roomId: %d not exist", roomId)
	}
	room := roomValue.(*bean.Room)
	value, ok := room.UserIdUserMap.Load(userId)
	fmt.Printf("userId: %v", ok)
	if !ok {
		return fmt.Errorf("userId:%v not exist", userId)
	}
	user := value.(*bean.User)
	user.Mu.Lock()
	defer user.Mu.Unlock()
	//1、需要解析出来，dtls参数给worker实现connect
	name, content := common_util.GetFingerNameContent(answer.Answer.Sdp)
	producerTransportId := user.Consumer.TransportId
	id := common_util.GneId()
	idStr := strconv.FormatInt(id, 10)
	msg := idStr + ":transport.connect:" + producerTransportId + ":{\"dtlsParameters\":{\"fingerprints\": [{\"algorithm\":\"" + name + "\",\"value\":\"" + content + "\"}],\"role\":\"client\"}}"
	workerMsgBytes := []byte(msg + "\000")
	raw := common_util.Encode(workerMsgBytes)
	if _, err = workerManager.ProducerSocket.Write(raw); err != nil {
		return err
	}
	//2、需要向worker发起consume请求 该步骤应该在transport connection之后再transport.consume
	remoteConsumer := user.Consumer.Consumers
	producerId := ""
	consumerId := ""
	for _, consumers := range remoteConsumer {
		for _, consumer := range consumers {
			transType := "simple"
			for _, encoding := range consumer.ConsumableRtpEncodings {
				if encoding.ScalabilityMode != "" {
					transType = "simulcast"
				}
			}
			consumerId = consumer.ConsumerId
			producerId = consumer.ProducerId
			kind := consumer.Kind
			encodings := consumer.ConsumableRtpEncodings
			parameters := consumer.RtpParameters
			req := make(map[string]interface{})
			req["consumerId"] = consumerId
			req["producerId"] = producerId
			req["kind"] = kind
			req["type"] = transType
			req["rtpParameters"] = parameters
			req["paused"] = true
			req["consumableRtpEncodings"] = encodings
			req["ignoreDtx"] = false
			marshal, _ := json.Marshal(req)
			id = common_util.GneId()
			idStr = strconv.FormatInt(id, 10)
			msg = idStr + ":transport.consume:" + producerTransportId + ":" + string(marshal)
			workerMsgBytes = []byte(msg + "\000")
			raw = common_util.Encode(workerMsgBytes)
			if _, err = workerManager.ProducerSocket.Write(raw); err != nil {
				return err
			}
			id = common_util.GneId()
			idStr = strconv.FormatInt(id, 10)
			msg = idStr + ":consumer.resume:" + consumerId + ":{}"
			workerMsgBytes = []byte(msg + "\000")
			raw = common_util.Encode(workerMsgBytes)
			if _, err = workerManager.ProducerSocket.Write(raw); err != nil {
				return err
			}
		}
	}

	return nil
}
