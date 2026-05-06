package bean

import (
	"github.com/gorilla/websocket"
	"sync"
)

type User struct {
	WorkerIdTransportIdMap map[int64]string  `json:"workerIdTransportId"`
	UserId                 string            `json:"userId"`
	RoomId                 string            `json:"roomId"`
	Conn                   *websocket.Conn   `json:"conn"`
	Producer               ProducerTransport `json:"producer"`
	Consumer               ConsumerTransport `json:"consumer"`
	Mu                     sync.Mutex
}

type ProducerTransport struct {
	TransportId    string                     `json:"transportId"`
	Dtls           Dtls                       `json:"dtls"`
	Ip             string                     `json:"ip"`
	Port           int                        `json:"port"`
	Fingerprint    string                     `json:"fingerprint"`
	FingerPassword string                     `json:"fingerPassword"`
	Producers      map[string]MediaParameters `json:"producers"`
}

type ConsumerTransport struct {
	TransportId    string                         `json:"transportId"`
	Dtls           Dtls                           `json:"dtls"`
	Ip             string                         `json:"ip"`
	Port           int                            `json:"port"`
	Fingerprint    string                         `json:"fingerprint"`
	FingerPassword string                         `json:"fingerPassword"`
	Consumers      map[string]map[string]Consumer `json:"consumers"`
}
