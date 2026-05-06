package common_util

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math/rand"
	"runtime/debug"
	"strings"
	"time"
)

type State int

var id int64

const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
const charset = "abcdefghijklmnopqrstuvwxyz0123456789"

const (
	SEPARATOR_SYMBOL        byte = ':'
	SYMBOL                  byte = ','
	START_SYMBOL_LEFT_BRACE byte = '{'
	END_SYMBOL_RIGHT_BRACE  byte = '}'

	MSG_RESP_MAX_LEN     = 4194304
	MSG_REQ_MAX_LEN      = 4194313
	BUFFER_SIZE      int = 2048

	PARSE_LENGTH State = iota
	PARSE_SEPARATOR
	PARSE_DATA
	PARSE_END
)

func Encode(payload []byte) (raw []byte) {
	length := len(payload)
	buffer := make([]byte, 0, len(payload)+4)
	buffer = binary.LittleEndian.AppendUint32(buffer, uint32(length))
	buffer = append(buffer, payload...)
	return buffer
}

type Decoder struct {
	parsedData []byte
	length     int
	state      State
	outputCh   chan []byte
}

func NewDecoder() *Decoder {
	return &Decoder{
		state:      PARSE_LENGTH,
		parsedData: []byte{},
		outputCh:   make(chan []byte, BUFFER_SIZE),
	}
}

func (decoder *Decoder) Reset() {
	decoder.length = 0
	decoder.parsedData = nil
	decoder.state = PARSE_LENGTH
}

func (decoder *Decoder) Length() int {
	return decoder.length
}

func (decoder *Decoder) Result() chan []byte {
	return decoder.outputCh
}

// Feed New incoming parsedData packets are feeded into the decoder using this method.
// Call this method every time we have a new set of parsedData.
func (decoder *Decoder) Feed(data []byte) {
	defer func() {
		if err := recover(); err != nil {
			decoder.Reset()
			fmt.Println("decode worker data panic:", debug.Stack())
		}
	}()
	var err error
	for i := 0; i < len(data); {
		i, err = decoder.parse(i, data)
		if err != nil {
			fmt.Printf("decoder worker data invalid, index:%d err:%v, msg:%s\n", i, err, data)
			break
		}
	}
	decoder.Reset()
}

func (decoder *Decoder) parse(i int, data []byte) (int, error) {
	switch decoder.state {
	case PARSE_LENGTH:
		return decoder.parseLengthForPre4Byte(i, data)
	case PARSE_DATA:
		return decoder.parseData(i, data)
	case PARSE_END:
		return decoder.parseEnd(i, data)
	}
	return i, nil
}
func (decoder *Decoder) parseEnd(i int, data []byte) (int, error) {
	// 校验数据结尾字符是否是右大括号，Symbol matches, that means this is valid data
	if data[i] == END_SYMBOL_RIGHT_BRACE {
		decoder.outputCh <- decoder.parsedData
	}
	// Irrespective of what symbol we got we have to reset.
	// Since we are looking for new data from now onwards.
	decoder.Reset()
	i++
	return i, nil
}
func (decoder *Decoder) parseData(i int, data []byte) (int, error) {
	//剩余未解析的数据总长度（可能有多条拼接的）
	dataSize := len(data) - i
	//判断数据长度部分大于剩余未解析的数据总长度，则认为数据异常，直接返回错误，结束解析
	if decoder.length > dataSize {
		return i, errors.New("data too large")
	}
	//计算本次解析的消息的数据长度（可能有多条拼接的，所以这个长度指的是本次解析那条消息的数据长度）
	dataLength := min(decoder.length, dataSize)
	//将数据部分赋值给parsedData
	decoder.parsedData = append(decoder.parsedData, data[i:i+dataLength]...)
	//判断解析的数据部分长度是否和真实拿到的数据长度一致，不一致则直接返回错误，结束解析
	decoder.length = decoder.length - dataLength
	if decoder.length == 0 {
		decoder.state = PARSE_END
	} else {
		return i + dataLength - 1, errors.New("data too large")
	}
	// We already parsed till i + dataLength
	// 此处解析修改为返回最后一个字符，是json的右大括号
	return i + dataLength - 1, nil
}
func (decoder *Decoder) parseLengthForPre4Byte(i int, data []byte) (int, error) {
	// 校验读到数据小于4位，或者大于4位但第5位不是'{'，则直接返回数据错误，结束解析
	if len(data[i:]) < 4 || (len(data[i:]) > 4 && data[i+4] != START_SYMBOL_LEFT_BRACE) {
		fmt.Printf("parse data length:%v\n", data[i:i+5])
		return i, errors.New("data too large")
	}
	dataLengthByte := data[i : i+4]
	dataLength := int(binary.LittleEndian.Uint32(dataLengthByte))
	// 校验如果前四字节的数据长度小于0或大于数据总长度，则认为数据长度不对，直接返回数据错误，结束解析
	if dataLength <= 0 || dataLength > len(data[i:]) || dataLength > MSG_RESP_MAX_LEN {
		return i, errors.New("ERR_DATA_LEN_INVALID_PARSELENGTH")
	}
	decoder.length = dataLength
	decoder.state = PARSE_DATA
	return i + 4, nil
}

func GneId() int64 {
	id = id + 1
	return id
}

func RandStringBytes(n int) string {
	rand.Seed(time.Now().UnixNano())
	b := make([]byte, n)
	for i := range b {
		b[i] = letterBytes[rand.Intn(len(letterBytes))]
	}
	return string(b)
}

func GetFingerNameContent(sdp string) (name, content string) {
	lines := strings.Split(sdp, "\r\n")

	for _, line := range lines {
		if strings.HasPrefix(line, "a=fingerprint:") {
			parts := strings.SplitN(line, "sha-", 2)
			if len(parts) != 2 {
				return "", ""
			}
			inter := strings.SplitN(parts[1], " ", 2)
			copy := inter[1]
			return "sha-" + inter[0], copy
		}
	}
	return "", ""
}

func RandomStringWithHyphen(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}

func GenerateMsid() string {
	return fmt.Sprintf("%s-%s-%s-%s-%s",
		RandomStringWithHyphen(8),
		RandomStringWithHyphen(4),
		RandomStringWithHyphen(4),
		RandomStringWithHyphen(4),
		RandomStringWithHyphen(12),
	)
}

func GenerateSsrc() uint32 {
	now := time.Now().UnixNano()
	buf := make([]byte, 4)
	_, err := rand.Read(buf)
	if err != nil {
		return uint32(now) ^ uint32(now>>32)
	}
	randomPart := binary.BigEndian.Uint32(buf)
	return uint32(now) ^ randomPart
}
