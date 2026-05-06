package common_util

import (
	"fmt"
	"mediasoup_server/bean"
	"strconv"
	"strings"
	"time"
)

func GenerateDownOfferSdp(algorithmName, algorithmValue string, user *bean.User) string {
	//通道参数
	icePwd := user.Consumer.FingerPassword
	iceUfrag := user.Consumer.Fingerprint
	msid := GenerateMsid()
	ip := user.Consumer.Ip
	port := user.Consumer.Port
	var udpCandidate = fmt.Sprintf("a=candidate:udpcandidate 1 udp 1076302079 %s %d typ host generation 0", ip, port)
	now := time.Now()
	unixNano := now.UnixNano()
	origin := fmt.Sprintf("o=- %d 2 IN IP4 127.0.0.1", unixNano)
	midSize := 0
	for _, consumers := range user.Consumer.Consumers {
		midSize += len(consumers)
	}
	var bundle = "a=group:BUNDLE"
	for i := range midSize {
		bundle += " " + strconv.FormatInt(int64(i), 10)
	}
	sdpLines := []string{
		"v=0",
		origin,
		"s=-",
		"t=0 0",
		bundle,
		fmt.Sprintf("a=msid-semantic: WMS %s", msid),
		"a=ice-lite",
	}
	i := 0
	for userId := range user.Consumer.Consumers {
		consumers := user.Consumer.Consumers[userId]
		audio := []string{}
		video := []string{}
		for consumerId := range consumers {
			consumer := consumers[consumerId]
			kind := consumer.Kind
			if kind == "audio" {
				audioSsrc := int(consumer.RtpParameters.Encodings[0].Ssrc)
				audioName := consumer.RtpParameters.Rtcp.Cname
				audio = append(audio, fmt.Sprintf("m=audio %d UDP/TLS/RTP/SAVPF 100", port))
				audio = append(audio, fmt.Sprintf("a=msid:%s %s", msid, audioName))
				audio = append(audio, fmt.Sprintf("a=mid:%d", 2*i))
				audio = append(audio, fmt.Sprintf("c=IN IP4 %s", ip))
				audio = append(audio, "a=rtcp:9 IN IP4 0.0.0.0")
				audio = append(audio, udpCandidate)
				audio = append(audio, fmt.Sprintf("a=ice-ufrag:%s", iceUfrag))
				audio = append(audio, fmt.Sprintf("a=ice-pwd:%s", icePwd))
				audio = append(audio, "a=ice-options:renomination")
				audio = append(audio, fmt.Sprintf("a=fingerprint:%s %s", algorithmName, algorithmValue))
				audio = append(audio, "a=setup:actpass")
				audio = append(audio, "a=extmap:1 urn:ietf:params:rtp-hdrext:sdes:mid")
				audio = append(audio, "a=extmap:4 http://www.webrtc.org/experiments/rtp-hdrext/abs-send-time")
				audio = append(audio, "a=extmap:5 http://www.webrtc.org/experiments/rtp-hdrext/playout-delay")
				audio = append(audio, "a=extmap:11 urn:ietf:params:rtp-hdrext:sdes:repaired-rtp-stream-id")
				audio = append(audio, "a=extmap:12 urn:ietf:params:rtp-hdrext:toffset")
				audio = append(audio, "a=sendonly")
				audio = append(audio, "a=rtcp-mux")
				audio = append(audio, "a=rtcp-rsize")
				audio = append(audio, "a=rtpmap:100 opus/48000/2")
				audio = append(audio, "a=rtcp-fb:100 transport-cc")
				audio = append(audio, "a=rtcp-fb:100 nack")
				audio = append(audio, fmt.Sprintf("a=ssrc:%d cname:%s", audioSsrc, audioName))
				audio = append(audio, fmt.Sprintf("a=ssrc:%d msid:%s audio-%s", audioSsrc, msid, strconv.FormatInt(int64(audioSsrc), 10)))
				consumer.RtpParameters.Mid = strconv.FormatInt(int64(2*i), 10)
			} else {
				//video = videoH264(algorithmName, algorithmValue, consumer, port, ip, udpCandidate, iceUfrag, icePwd, i, msid)
				video = videoVP8(algorithmName, algorithmValue, consumer, port, ip, udpCandidate, iceUfrag, icePwd, i, msid)
			}
			consumers[consumerId] = consumer
		}
		i++
		if len(audio) > 0 {
			sdpLines = append(sdpLines, audio...)
		}
		if len(video) > 0 {
			sdpLines = append(sdpLines, video...)
		}
	}

	sdpLines = append(sdpLines)

	return strings.Join(sdpLines, "\r\n") + "\r\n"
}
func GenerateDownOfferSdpPipe(algorithmName, algorithmValue string, user *bean.User, audio_ssrc, video_ssrc uint32) string {
	//通道参数
	icePwd := user.Consumer.FingerPassword
	iceUfrag := user.Consumer.Fingerprint
	msid := GenerateMsid()
	ip := user.Consumer.Ip
	port := user.Consumer.Port
	var udpCandidate = fmt.Sprintf("a=candidate:udpcandidate 1 udp 1076302079 %s %d typ host generation 0", ip, port)
	now := time.Now()
	unixNano := now.UnixNano()
	origin := fmt.Sprintf("o=- %d 2 IN IP4 127.0.0.1", unixNano)

	var bundle = "a=group:BUNDLE 0 1"

	sdpLines := []string{
		"v=0",
		origin,
		"s=-",
		"t=0 0",
		bundle,
		fmt.Sprintf("a=msid-semantic: WMS %s", msid),
		"a=ice-lite",
		fmt.Sprintf("m=audio %d UDP/TLS/RTP/SAVPF 100", port),
		fmt.Sprintf("a=msid:%s %s", msid, "fdasdfasf"),
		fmt.Sprintf("a=mid:%d", 0),
		fmt.Sprintf("c=IN IP4 %s", ip),
		"a=rtcp:9 IN IP4 0.0.0.0",
		udpCandidate,
		fmt.Sprintf("a=ice-ufrag:%s", iceUfrag),
		fmt.Sprintf("a=ice-pwd:%s", icePwd),
		"a=ice-options:renomination",
		fmt.Sprintf("a=fingerprint:%s %s", algorithmName, algorithmValue),
		"a=setup:actpass",
		"a=extmap:1 urn:ietf:params:rtp-hdrext:sdes:mid",
		"a=extmap:4 http://www.webrtc.org/experiments/rtp-hdrext/abs-send-time",
		"a=extmap:5 http://www.webrtc.org/experiments/rtp-hdrext/playout-delay",
		"a=extmap:11 urn:ietf:params:rtp-hdrext:sdes:repaired-rtp-stream-id",
		"a=extmap:12 urn:ietf:params:rtp-hdrext:toffset",
		"a=sendonly",
		"a=rtcp-mux",
		"a=rtcp-rsize",
		"a=rtpmap:100 opus/48000/2",
		"a=rtcp-fb:100 transport-cc",
		"a=rtcp-fb:100 nack",
		fmt.Sprintf("a=ssrc:%d cname:%s", audio_ssrc, "audio-pipe"),
		fmt.Sprintf("a=ssrc:%d msid:%s audio-%s", audio_ssrc, msid, strconv.FormatInt(int64(audio_ssrc), 10)),
		fmt.Sprintf("m=video %d UDP/TLS/RTP/SAVPF 101 102", port),
		fmt.Sprintf("c=IN IP4 %s", ip),
		"a=rtcp:9 IN IP4 0.0.0.0",
		udpCandidate,
		fmt.Sprintf("a=ice-ufrag:%s", iceUfrag),
		fmt.Sprintf("a=ice-pwd:%s", icePwd),
		"a=ice-options:renomination",
		fmt.Sprintf("a=fingerprint:%s %s", algorithmName, algorithmValue),
		"a=setup:actpass",
		fmt.Sprintf("a=mid:%d", 1),
		"a=extmap:1 urn:ietf:params:rtp-hdrext:sdes:mid",
		"a=extmap:4 http://www.webrtc.org/experiments/rtp-hdrext/abs-send-time",
		"a=extmap:5 http://www.webrtc.org/experiments/rtp-hdrext/playout-delay",
		"a=extmap:11 urn:ietf:params:rtp-hdrext:sdes:repaired-rtp-stream-id",
		"a=extmap:12 urn:ietf:params:rtp-hdrext:toffset",
		"a=sendonly",
		fmt.Sprintf("a=msid:%s %s", msid, "video-pipe"),
		"a=extmap-allow-mixed",
		"a=rtcp-mux",
		"a=rtcp-rsize",
		"a=rtpmap:101 VP8/90000",
		"a=rtcp-fb:101 transport-cc",
		"a=rtcp-fb:101 ccm fir",
		"a=rtcp-fb:101 nack",
		"a=rtcp-fb:101 nack pli",
		"a=rtpmap:102 rtx/90000",
		"a=fmtp:102 apt=101",
		fmt.Sprintf("a=ssrc-group:FID %d %d", video_ssrc, video_ssrc+1),
		fmt.Sprintf("a=ssrc:%d cname:%s", video_ssrc, "video-pipe"),
		fmt.Sprintf("a=ssrc:%d cname:%s", video_ssrc+1, "video-pipe"),
	}
	sdpLines = append(sdpLines)
	return strings.Join(sdpLines, "\r\n") + "\r\n"
}
func videoVP8(algorithmName string, algorithmValue string, consumer bean.Consumer, port int, ip string, udpCandidate string, iceUfrag string, icePwd string, i int, msid string) []string {
	video := []string{}
	videoSsrc1 := int(consumer.RtpParameters.Encodings[0].Ssrc)
	videoSsrc2 := int(consumer.RtpParameters.Encodings[0].Rtx["ssrc"])
	videoName := consumer.RtpParameters.Rtcp.Cname
	video = append(video, fmt.Sprintf("m=video %d UDP/TLS/RTP/SAVPF 101 102", port))
	video = append(video, fmt.Sprintf("c=IN IP4 %s", ip))
	video = append(video, "a=rtcp:9 IN IP4 0.0.0.0")
	video = append(video, udpCandidate)
	video = append(video, fmt.Sprintf("a=ice-ufrag:%s", iceUfrag))
	video = append(video, fmt.Sprintf("a=ice-pwd:%s", icePwd))
	video = append(video, "a=ice-options:renomination")
	video = append(video, fmt.Sprintf("a=fingerprint:%s %s", algorithmName, algorithmValue))
	video = append(video, "a=setup:actpass")
	video = append(video, fmt.Sprintf("a=mid:%d", 2*i+1))
	video = append(video, "a=extmap:1 urn:ietf:params:rtp-hdrext:sdes:mid")
	video = append(video, "a=extmap:4 http://www.webrtc.org/experiments/rtp-hdrext/abs-send-time")
	video = append(video, "a=extmap:5 http://www.webrtc.org/experiments/rtp-hdrext/playout-delay")
	video = append(video, "a=extmap:11 urn:ietf:params:rtp-hdrext:sdes:repaired-rtp-stream-id")
	video = append(video, "a=extmap:12 urn:ietf:params:rtp-hdrext:toffset")
	video = append(video, "a=sendonly")
	video = append(video, fmt.Sprintf("a=msid:%s %s", msid, videoName))
	video = append(video, "a=extmap-allow-mixed")
	video = append(video, "a=rtcp-mux")
	video = append(video, "a=rtcp-rsize")
	video = append(video, "a=rtpmap:101 VP8/90000")
	video = append(video, "a=rtcp-fb:101 transport-cc")
	video = append(video, "a=rtcp-fb:101 ccm fir")
	video = append(video, "a=rtcp-fb:101 nack")
	video = append(video, "a=rtcp-fb:101 nack pli")
	video = append(video, "a=rtpmap:102 rtx/90000")
	video = append(video, "a=fmtp:102 apt=101")
	video = append(video, fmt.Sprintf("a=ssrc-group:FID %d %d", videoSsrc1, videoSsrc2))
	video = append(video, fmt.Sprintf("a=ssrc:%d cname:%s", videoSsrc1, videoName))
	video = append(video, fmt.Sprintf("a=ssrc:%d cname:%s", videoSsrc2, videoName))
	consumer.RtpParameters.Mid = strconv.FormatInt(int64(2*i+1), 10)
	return video
}

func videoH264(algorithmName string, algorithmValue string, consumer bean.Consumer, port int, ip string, udpCandidate string, iceUfrag string, icePwd string, i int, msid string) []string {
	video := []string{}
	videoSsrc1 := int(consumer.RtpParameters.Encodings[0].Ssrc)
	videoSsrc2 := int(consumer.RtpParameters.Encodings[0].Rtx["ssrc"])
	videoName := consumer.RtpParameters.Rtcp.Cname
	video = append(video, fmt.Sprintf("m=video %d UDP/TLS/RTP/SAVPF 101 102", port))
	video = append(video, fmt.Sprintf("c=IN IP4 %s", ip))
	video = append(video, "a=rtcp:9 IN IP4 0.0.0.0")
	video = append(video, udpCandidate)
	video = append(video, fmt.Sprintf("a=ice-ufrag:%s", iceUfrag))
	video = append(video, fmt.Sprintf("a=ice-pwd:%s", icePwd))
	video = append(video, "a=ice-options:renomination")
	video = append(video, fmt.Sprintf("a=fingerprint:%s %s", algorithmName, algorithmValue))
	video = append(video, "a=setup:actpass")
	video = append(video, fmt.Sprintf("a=mid:%d", 2*i+1))
	video = append(video, "a=extmap:1 urn:ietf:params:rtp-hdrext:sdes:mid")
	video = append(video, "a=extmap:4 http://www.webrtc.org/experiments/rtp-hdrext/abs-send-time")
	video = append(video, "a=extmap:5 http://www.webrtc.org/experiments/rtp-hdrext/playout-delay")
	video = append(video, "a=extmap:11 urn:ietf:params:rtp-hdrext:sdes:repaired-rtp-stream-id")
	video = append(video, "a=extmap:12 urn:ietf:params:rtp-hdrext:toffset")
	video = append(video, "a=sendonly")
	video = append(video, fmt.Sprintf("a=msid:%s %s", msid, videoName))
	video = append(video, "a=extmap-allow-mixed")
	video = append(video, "a=rtcp-mux")
	video = append(video, "a=rtcp-rsize")
	video = append(video, "a=rtpmap:101 H264/90000")
	video = append(video, "a=rtcp-fb:101 transport-cc")
	video = append(video, "a=rtcp-fb:101 ccm fir")
	video = append(video, "a=rtcp-fb:101 nack")
	video = append(video, "a=rtcp-fb:101 nack pli")
	video = append(video, "a=rtpmap:102 rtx/90000")
	video = append(video, "a=fmtp:102 apt=101")
	video = append(video, fmt.Sprintf("a=ssrc-group:FID %d %d", videoSsrc1, videoSsrc2))
	video = append(video, fmt.Sprintf("a=ssrc:%d cname:%s", videoSsrc1, videoName))
	video = append(video, fmt.Sprintf("a=ssrc:%d cname:%s", videoSsrc2, videoName))
	consumer.RtpParameters.Mid = strconv.FormatInt(int64(2*i+1), 10)
	return video
}
