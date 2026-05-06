package bean

import "sync"

type SignalMessage struct {
	Type   string `json:"type"`
	RoomId string `json:"roomId"`
	UserId string `json:"userId"`
}
type AnswerReq struct {
	SignalMessage
	Answer Sdp `json:"answer"`
}

type OfferReq struct {
	SignalMessage
	Offer Sdp `json:"offer"`
}
type Sdp struct {
	Sdp  string `json:"sdp"`
	Type string `json:"type"`
}
type IceCandidate struct {
	Foundation string `json:"foundation"`
	Component  int
	Protocol   string `json:"protocol"`
	Priority   uint32 `json:"priority"`
	IP         string `json:"ip"`
	Port       int    `json:"port"`
	Type       string `json:"type"`
}

type DtlsParameters struct {
	Fingerprints []struct {
		Algorithm string `json:"algorithm"`
		Value     string `json:"value"`
	} `json:"fingerprints"`
}
type IceParameters struct {
	IceLite          bool   `json:"iceLite"`
	Password         string `json:"password"`
	UsernameFragment string `json:"usernameFragment"`
}
type ResponseData struct {
	DtlsParameters DtlsParameters `json:"dtlsParameters"`
	IceCandidates  []IceCandidate `json:"iceCandidates"`
	IceParameters  IceParameters  `json:"iceParameters"`
}
type Dtls struct {
	Data ResponseData `json:"data"`
}

type MediaParameters struct {
	ProducerId    string        `json:"producerId,omitempty"`
	Kind          string        `json:"kind,omitempty"`
	RtpParameters RtpParameters `json:"rtpParameters,omitempty"`
	RtpMapping    RtpMapping    `json:"rtpMapping,omitempty"`
	Paused        bool          `json:"paused"`
}
type Codec struct {
	MimeType     string                 `json:"mimeType,omitempty"`
	PayloadType  int                    `json:"payloadType,omitempty"`
	ClockRate    int                    `json:"clockRate,omitempty"`
	Channels     int                    `json:"channels,omitempty"`
	Parameters   map[string]interface{} `json:"parameters,omitempty"`
	RtcpFeedback []RtcpFeedback         `json:"rtcpFeedback,omitempty"`
}
type RtcpFeedback struct {
	Type      string `json:"type,omitempty"`
	Parameter string `json:"parameter,omitempty"`
}
type Encoding struct {
	Active                bool              `json:"active,omitempty"`
	Ssrc                  uint32            `json:"ssrc,omitempty"`
	Rid                   string            `json:"rid,omitempty"`
	MaxBitrate            int               `json:"maxBitrate,omitempty"`
	ScalabilityMode       string            `json:"scalabilityMode,omitempty"`
	ScaleResolutionDownBy int               `json:"scaleResolutionDownBy,omitempty"`
	Dtx                   bool              `json:"dtx"`
	Rtx                   map[string]uint32 `json:"rtx,omitempty"`
}
type HeaderExtension struct {
	Uri        string                 `json:"uri,omitempty"`
	Id         int                    `json:"id,omitempty"`
	Encrypt    bool                   `json:"encrypt"`
	Parameters map[string]interface{} `json:"parameters"`
}
type Rtcp struct {
	Cname       string `json:"cname,omitempty"`
	ReducedSize bool   `json:"reducedSize,omitempty"`
	Mux         bool   `json:"mux,omitempty"`
}
type RtpMapping struct {
	Codecs    []CodecMapping    `json:"codecs,omitempty"`
	Encodings []EncodingMapping `json:"encodings,omitempty"`
}
type CodecMapping struct {
	PayloadType       int `json:"payloadType,omitempty"`
	MappedPayloadType int `json:"mappedPayloadType,omitempty"`
}
type EncodingMapping struct {
	Ssrc            uint32 `json:"ssrc,omitempty"`
	ScalabilityMode string `json:"scalabilityMode,omitempty"`
	MappedSsrc      uint32 `json:"mappedSsrc,omitempty"`
	Rid             string `json:"rid,omitempty,omitempty"`
}
type RtpParameters struct {
	Mid              string            `json:"mid,omitempty"`
	Codecs           []Codec           `json:"codecs,omitempty"`
	HeaderExtensions []HeaderExtension `json:"headerExtensions,omitempty"`
	Encodings        []Encoding        `json:"encodings,omitempty"`
	Rtcp             Rtcp              `json:"rtcp,omitempty"`
}

//消费请求结构体

type Consumer struct {
	ProducerId             string                   `json:"producerId"`
	Type                   string                   `json:"type"`
	ConsumerId             string                   `json:"consumerId"`
	Kind                   string                   `json:"kind"`
	RtpParameters          RtpParameters            `json:"rtpParameters"`
	ConsumableRtpEncodings []ConsumableRtpEncodings `json:"consumableRtpEncodings"`
}
type ConsumableRtpEncodings struct {
	Ssrc                  uint32 `json:"ssrc"`
	Active                bool   `json:"active,omitempty"`
	MaxBitrate            int    `json:"maxBitrate,omitempty"`
	ScaleResolutionDownBy int    `json:"scaleResolutionDownBy,omitempty"`
	ScalabilityMode       string `json:"scalabilityMode,omitempty"`
	Dtx                   bool   `json:"dtx,omitempty"`
	Mux                   bool   `json:"mux,omitempty"`
}

type Sha struct {
	Algorithm string `json:"algorithm"`
	Value     string `json:"value"`
}

type Room struct {
	ActiveSpeakerObserverId string   `json:"activeSpeakerObserverId"`
	AudioLevelObserverId    string   `json:"audioLevelObserverId"`
	UserIdUserMap           sync.Map `json:"userIdUserMap"`
}
