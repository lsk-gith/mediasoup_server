package common_util

import (
	"fmt"
	"mediasoup_server/bean"
	"strings"
	"time"
)

// 生成上行answerSdp
func GenerateUpAnswerSDP(
	algorithmName string,
	algorithmValue string,
	user *bean.User,
	offerSdp string,
) string {
	audioMid, videoMid := GetOfferSDPMid(offerSdp)
	icePwd := user.Producer.FingerPassword
	iceUfrag := user.Producer.Fingerprint
	typeTrans := "simple"
	for _, parameters := range user.Producer.Producers {
		kind := parameters.Kind
		if kind == "video" {
			for _, encoding := range parameters.RtpParameters.Encodings {
				if encoding.Rid != "" {
					typeTrans = "simulcast"
				}
			}
		}
	}
	if typeTrans == "simple" {
		return genSimpleVP8(icePwd, iceUfrag, algorithmName, algorithmValue, user, audioMid, videoMid)
		//if user.UserId == "34123" {
		//	return genSimpleH264ForPython(icePwd, iceUfrag, algorithmName, algorithmValue, user, audioMid, videoMid)
		//} else {
		//	return genSimpleH264(icePwd, iceUfrag, algorithmName, algorithmValue, user, audioMid, videoMid)
		//}
	} else if typeTrans == "simulcast" {
		return genSimulcastVP8(icePwd, iceUfrag, algorithmName, algorithmValue, user, audioMid, videoMid)
		//return genSimulcastH264(icePwd, iceUfrag, algorithmName, algorithmValue, user, audioMid, videoMid)
	} else {
		return ""
	}

}

func genSimulcastH264(icePwd, iceUfrag, algorithmName, algorithmValue string, user *bean.User, audioMid, videoMid string) string {
	now := time.Now()
	unixNano := now.UnixNano()
	origin := fmt.Sprintf("o=- %d 2 IN IP4 127.0.0.1", unixNano)
	ip := user.Producer.Ip
	port := user.Producer.Port
	var udpCandidate = fmt.Sprintf("a=candidate:udpcandidate 1 udp 1076302079 %s %d typ host generation 0", ip, port)

	sdpLines := []string{
		"v=0",
		origin,
		"s=-",
		"t=0 0",
		fmt.Sprintf("a=group:BUNDLE %s %s", audioMid, videoMid),
		"a=msid-semantic: WMS",
		//"a=ice-lite",
		fmt.Sprintf("m=audio %d UDP/TLS/RTP/SAVPF 111", port),
		fmt.Sprintf("c=IN IP4 %s", ip),
		"a=rtcp:9 IN IP4 0.0.0.0",
		udpCandidate,
		fmt.Sprintf("a=ice-ufrag:%s", iceUfrag),
		fmt.Sprintf("a=ice-pwd:%s", icePwd),
		"a=ice-options:renomination",
		fmt.Sprintf("a=fingerprint:%s %s", algorithmName, algorithmValue),
		"a=setup:active",
		fmt.Sprintf("a=mid:%s", audioMid),
		"a=extmap:4 urn:ietf:params:rtp-hdrext:sdes:mid",
		"a=extmap:2 http://www.webrtc.org/experiments/rtp-hdrext/abs-send-time",
		"a=extmap:3 http://www.ietf.org/id/draft-holmer-rmcat-transport-wide-cc-extensions-01",
		"a=extmap:1 urn:ietf:params:rtp-hdrext:ssrc-audio-level",
		"a=recvonly",
		"a=rtcp-mux",
		"a=rtcp-rsize",
		"a=rtpmap:111 opus/48000/2",
		"a=rtcp-fb:111 transport-cc",
		"a=rtcp-fb:111 nack",
		"a=fmtp:111 stereo=1;usedtx=1;useinbandfec=1",
		fmt.Sprintf("m=video %d UDP/TLS/RTP/SAVPF 96 97", port),
		fmt.Sprintf("c=IN IP4 %s", ip),
		"a=rtcp:9 IN IP4 0.0.0.0",
		udpCandidate,
		fmt.Sprintf("a=ice-ufrag:%s", iceUfrag),
		fmt.Sprintf("a=ice-pwd:%s", icePwd),
		"a=ice-options:renomination",
		fmt.Sprintf("a=fingerprint:%s %s", algorithmName, algorithmValue),
		"a=setup:active",
		fmt.Sprintf("a=mid:%s", videoMid),
		"a=extmap:4 urn:ietf:params:rtp-hdrext:sdes:mid",
		"a=extmap:10 urn:ietf:params:rtp-hdrext:sdes:rtp-stream-id",
		"a=extmap:11 urn:ietf:params:rtp-hdrext:sdes:repaired-rtp-stream-id",
		"a=extmap:2 http://www.webrtc.org/experiments/rtp-hdrext/abs-send-time",
		"a=extmap:3 http://www.ietf.org/id/draft-holmer-rmcat-transport-wide-cc-extensions-01",
		"a=extmap:13 urn:3gpp:video-orientation",
		"a=extmap:14 urn:ietf:params:rtp-hdrext:toffset",
		"a=recvonly",
		"a=rtcp-mux",
		"a=rtcp-rsize",
		"a=rtpmap:96 H264/90000",
		"a=rtcp-fb:96 transport-cc",
		"a=rtcp-fb:96 ccm fir",
		"a=rtcp-fb:96 nack",
		"a=rtcp-fb:96 nack pli",
		"a=fmtp:96 x-google-start-bitrate=1000",
		"a=rtpmap:97 rtx/90000",
		"a=fmtp:97 apt=96",
		"a=rid:r0 recv",
		"a=rid:r1 recv",
		"a=rid:r2 recv",
		"a=simulcast:recv r0;r1;r2",
	}

	// Add ICE candidates
	sdpLines = append(sdpLines)

	return strings.Join(sdpLines, "\r\n") + "\r\n"
}

func genSimulcastVP8(icePwd, iceUfrag, algorithmName, algorithmValue string, user *bean.User, audioMid, videoMid string) string {
	now := time.Now()
	unixNano := now.UnixNano()
	origin := fmt.Sprintf("o=- %d 2 IN IP4 127.0.0.1", unixNano)
	ip := user.Producer.Ip
	port := user.Producer.Port
	var udpCandidate = fmt.Sprintf("a=candidate:udpcandidate 1 udp 1076302079 %s %d typ host generation 0", ip, port)

	sdpLines := []string{
		"v=0",
		origin,
		"s=-",
		"t=0 0",
		fmt.Sprintf("a=group:BUNDLE %s %s", audioMid, videoMid),
		"a=msid-semantic: WMS",
		//"a=ice-lite",
		fmt.Sprintf("m=audio %d UDP/TLS/RTP/SAVPF 111", port),
		fmt.Sprintf("c=IN IP4 %s", ip),
		"a=rtcp:9 IN IP4 0.0.0.0",
		udpCandidate,
		fmt.Sprintf("a=ice-ufrag:%s", iceUfrag),
		fmt.Sprintf("a=ice-pwd:%s", icePwd),
		"a=ice-options:renomination",
		fmt.Sprintf("a=fingerprint:%s %s", algorithmName, algorithmValue),
		"a=setup:active",
		fmt.Sprintf("a=mid:%s", audioMid),
		"a=extmap:4 urn:ietf:params:rtp-hdrext:sdes:mid",
		"a=extmap:2 http://www.webrtc.org/experiments/rtp-hdrext/abs-send-time",
		"a=extmap:3 http://www.ietf.org/id/draft-holmer-rmcat-transport-wide-cc-extensions-01",
		"a=extmap:1 urn:ietf:params:rtp-hdrext:ssrc-audio-level",
		"a=recvonly",
		"a=rtcp-mux",
		"a=rtcp-rsize",
		"a=rtpmap:111 opus/48000/2",
		"a=rtcp-fb:111 transport-cc",
		"a=rtcp-fb:111 nack",
		"a=fmtp:111 stereo=1;usedtx=1;useinbandfec=1",
		fmt.Sprintf("m=video %d UDP/TLS/RTP/SAVPF 96 97", port),
		fmt.Sprintf("c=IN IP4 %s", ip),
		"a=rtcp:9 IN IP4 0.0.0.0",
		udpCandidate,
		fmt.Sprintf("a=ice-ufrag:%s", iceUfrag),
		fmt.Sprintf("a=ice-pwd:%s", icePwd),
		"a=ice-options:renomination",
		fmt.Sprintf("a=fingerprint:%s %s", algorithmName, algorithmValue),
		"a=setup:active",
		fmt.Sprintf("a=mid:%s", videoMid),
		"a=extmap:4 urn:ietf:params:rtp-hdrext:sdes:mid",
		"a=extmap:10 urn:ietf:params:rtp-hdrext:sdes:rtp-stream-id",
		"a=extmap:11 urn:ietf:params:rtp-hdrext:sdes:repaired-rtp-stream-id",
		"a=extmap:2 http://www.webrtc.org/experiments/rtp-hdrext/abs-send-time",
		"a=extmap:3 http://www.ietf.org/id/draft-holmer-rmcat-transport-wide-cc-extensions-01",
		"a=extmap:13 urn:3gpp:video-orientation",
		"a=extmap:14 urn:ietf:params:rtp-hdrext:toffset",
		"a=recvonly",
		"a=rtcp-mux",
		"a=rtcp-rsize",
		"a=rtpmap:96 VP8/90000",
		"a=rtcp-fb:96 transport-cc",
		"a=rtcp-fb:96 ccm fir",
		"a=rtcp-fb:96 nack",
		"a=rtcp-fb:96 nack pli",
		"a=fmtp:96 x-google-start-bitrate=1000",
		"a=rtpmap:97 rtx/90000",
		"a=fmtp:97 apt=96",
		"a=rid:r0 recv",
		"a=rid:r1 recv",
		"a=rid:r2 recv",
		"a=simulcast:recv r0;r1;r2",
	}

	// Add ICE candidates
	sdpLines = append(sdpLines)

	return strings.Join(sdpLines, "\r\n") + "\r\n"
}
func genSimpleVP8(icePwd, iceUfrag, algorithmName, algorithmValue string, user *bean.User, audioMid, videoMid string) string {
	now := time.Now()
	unixNano := now.UnixNano()
	origin := fmt.Sprintf("o=- %d 2 IN IP4 127.0.0.1", unixNano)
	ip := user.Producer.Ip
	port := user.Producer.Port
	var udpCandidate = fmt.Sprintf("a=candidate:udpcandidate 1 udp 1076302079 %s %d typ host generation 0", ip, port)
	sdpLines := []string{
		"v=0",
		origin,
		"s=-",
		"t=0 0",
		fmt.Sprintf("a=group:BUNDLE %s %s", audioMid, videoMid),
		"a=msid-semantic: WMS",
		//"a=ice-lite",
		fmt.Sprintf("m=audio %d UDP/TLS/RTP/SAVPF 111", port),
		fmt.Sprintf("c=IN IP4 %s", ip),
		"a=rtcp:9 IN IP4 0.0.0.0",
		udpCandidate,
		fmt.Sprintf("a=ice-ufrag:%s", iceUfrag),
		fmt.Sprintf("a=ice-pwd:%s", icePwd),
		"a=ice-options:renomination",
		fmt.Sprintf("a=fingerprint:%s %s", algorithmName, algorithmValue),
		"a=setup:active",
		fmt.Sprintf("a=mid:%s", audioMid),
		"a=extmap:4 urn:ietf:params:rtp-hdrext:sdes:mid",
		"a=extmap:2 http://www.webrtc.org/experiments/rtp-hdrext/abs-send-time",
		"a=extmap:3 http://www.ietf.org/id/draft-holmer-rmcat-transport-wide-cc-extensions-01",
		"a=extmap:1 urn:ietf:params:rtp-hdrext:ssrc-audio-level",
		"a=recvonly",
		"a=rtcp-mux",
		"a=rtcp-rsize",
		"a=rtpmap:111 opus/48000/2",
		"a=rtcp-fb:111 transport-cc",
		"a=rtcp-fb:111 nack",
		"a=fmtp:111 stereo=1;usedtx=1;useinbandfec=1",
		fmt.Sprintf("m=video %d UDP/TLS/RTP/SAVPF 96 97", port),
		fmt.Sprintf("c=IN IP4 %s", ip),
		"a=rtcp:9 IN IP4 0.0.0.0",
		udpCandidate,
		fmt.Sprintf("a=ice-ufrag:%s", iceUfrag),
		fmt.Sprintf("a=ice-pwd:%s", icePwd),
		"a=ice-options:renomination",
		fmt.Sprintf("a=fingerprint:%s %s", algorithmName, algorithmValue),
		"a=setup:active",
		fmt.Sprintf("a=mid:%s", videoMid),
		"a=extmap:4 urn:ietf:params:rtp-hdrext:sdes:mid",
		"a=extmap:10 urn:ietf:params:rtp-hdrext:sdes:rtp-stream-id",
		"a=extmap:11 urn:ietf:params:rtp-hdrext:sdes:repaired-rtp-stream-id",
		"a=extmap:2 http://www.webrtc.org/experiments/rtp-hdrext/abs-send-time",
		"a=extmap:3 http://www.ietf.org/id/draft-holmer-rmcat-transport-wide-cc-extensions-01",
		"a=extmap:13 urn:3gpp:video-orientation",
		"a=extmap:14 urn:ietf:params:rtp-hdrext:toffset",
		"a=extmap:5 http://www.webrtc.org/experiments/rtp-hdrext/playout-delay",
		"a=recvonly",
		"a=rtcp-mux",
		"a=rtcp-rsize",
		"a=rtpmap:96 VP8/90000",
		"a=rtcp-fb:96 transport-cc",
		"a=rtcp-fb:96 ccm fir",
		"a=rtcp-fb:96 nack",
		"a=rtcp-fb:96 nack pli",
		"a=fmtp:96 x-google-start-bitrate=1000",
		"a=rtpmap:97 rtx/90000",
		"a=fmtp:97 apt=96",
		"a=simple",
	}

	// Add ICE candidates
	sdpLines = append(sdpLines)

	return strings.Join(sdpLines, "\r\n") + "\r\n"
}

func genSimpleH264(icePwd, iceUfrag, algorithmName, algorithmValue string, user *bean.User, audioMid, videoMid string) string {
	now := time.Now()
	unixNano := now.UnixNano()
	origin := fmt.Sprintf("o=- %d 2 IN IP4 127.0.0.1", unixNano)
	ip := user.Producer.Ip
	port := user.Producer.Port
	var udpCandidate = fmt.Sprintf("a=candidate:udpcandidate 1 udp 1076302079 %s %d typ host generation 0", ip, port)
	sdpLines := []string{
		"v=0",
		origin,
		"s=-",
		"t=0 0",
		fmt.Sprintf("a=group:BUNDLE %s %s", audioMid, videoMid),
		"a=msid-semantic: WMS",
		//"a=ice-lite",
		fmt.Sprintf("m=audio %d UDP/TLS/RTP/SAVPF 111", port),
		fmt.Sprintf("c=IN IP4 %s", ip),
		"a=rtcp:9 IN IP4 0.0.0.0",
		udpCandidate,
		fmt.Sprintf("a=ice-ufrag:%s", iceUfrag),
		fmt.Sprintf("a=ice-pwd:%s", icePwd),
		"a=ice-options:renomination",
		fmt.Sprintf("a=fingerprint:%s %s", algorithmName, algorithmValue),
		"a=setup:active",
		fmt.Sprintf("a=mid:%s", audioMid),
		"a=extmap:4 urn:ietf:params:rtp-hdrext:sdes:mid",
		"a=extmap:2 http://www.webrtc.org/experiments/rtp-hdrext/abs-send-time",
		"a=extmap:3 http://www.ietf.org/id/draft-holmer-rmcat-transport-wide-cc-extensions-01",
		"a=extmap:1 urn:ietf:params:rtp-hdrext:ssrc-audio-level",
		"a=recvonly",
		"a=rtcp-mux",
		"a=rtcp-rsize",
		"a=rtpmap:111 opus/48000/2",
		"a=rtcp-fb:111 transport-cc",
		"a=rtcp-fb:111 nack",
		"a=fmtp:111 stereo=1;usedtx=1;useinbandfec=1",
		fmt.Sprintf("m=video %d UDP/TLS/RTP/SAVPF 96 97", port),
		fmt.Sprintf("c=IN IP4 %s", ip),
		"a=rtcp:9 IN IP4 0.0.0.0",
		udpCandidate,
		fmt.Sprintf("a=ice-ufrag:%s", iceUfrag),
		fmt.Sprintf("a=ice-pwd:%s", icePwd),
		"a=ice-options:renomination",
		fmt.Sprintf("a=fingerprint:%s %s", algorithmName, algorithmValue),
		"a=setup:active",
		fmt.Sprintf("a=mid:%s", videoMid),
		"a=extmap:4 urn:ietf:params:rtp-hdrext:sdes:mid",
		"a=extmap:10 urn:ietf:params:rtp-hdrext:sdes:rtp-stream-id",
		"a=extmap:11 urn:ietf:params:rtp-hdrext:sdes:repaired-rtp-stream-id",
		"a=extmap:2 http://www.webrtc.org/experiments/rtp-hdrext/abs-send-time",
		"a=extmap:3 http://www.ietf.org/id/draft-holmer-rmcat-transport-wide-cc-extensions-01",
		"a=extmap:13 urn:3gpp:video-orientation",
		"a=extmap:14 urn:ietf:params:rtp-hdrext:toffset",
		"a=extmap:5 http://www.webrtc.org/experiments/rtp-hdrext/playout-delay",
		"a=recvonly",
		"a=rtcp-mux",
		"a=rtcp-rsize",
		"a=rtpmap:96 H264/90000",
		"a=rtcp-fb:96 transport-cc",
		"a=rtcp-fb:96 ccm fir",
		"a=rtcp-fb:96 nack",
		"a=rtcp-fb:96 nack pli",
		"a=fmtp:96 x-google-start-bitrate=1000",
		"a=rtpmap:97 rtx/90000",
		"a=fmtp:97 apt=96",
		"a=simple",
	}

	// Add ICE candidates
	sdpLines = append(sdpLines)

	return strings.Join(sdpLines, "\r\n") + "\r\n"
}

func genSimpleH264ForPython(icePwd, iceUfrag, algorithmName, algorithmValue string, user *bean.User, audioMid, videoMid string) string {
	now := time.Now()
	unixNano := now.UnixNano()
	origin := fmt.Sprintf("o=- %d 2 IN IP4 127.0.0.1", unixNano)
	ip := user.Producer.Ip
	port := user.Producer.Port
	var udpCandidate = fmt.Sprintf("a=candidate:udpcandidate 1 udp 1076302079 %s %d typ host generation 0", ip, port)

	sdpLines := []string{
		"v=0",
		origin,
		"s=-",
		"t=0 0",
		fmt.Sprintf("a=group:BUNDLE %s %s", audioMid, videoMid),
		"a=msid-semantic: WMS",
		//"a=ice-lite",
		fmt.Sprintf("m=audio %d UDP/TLS/RTP/SAVPF 111", port),
		fmt.Sprintf("c=IN IP4 %s", ip),
		"a=rtcp:9 IN IP4 0.0.0.0",
		udpCandidate,
		fmt.Sprintf("a=ice-ufrag:%s", iceUfrag),
		fmt.Sprintf("a=ice-pwd:%s", icePwd),
		"a=ice-options:renomination",
		fmt.Sprintf("a=fingerprint:%s %s", algorithmName, algorithmValue),
		"a=setup:active",
		fmt.Sprintf("a=mid:%s", audioMid),
		"a=extmap:4 urn:ietf:params:rtp-hdrext:sdes:mid",
		"a=extmap:2 http://www.webrtc.org/experiments/rtp-hdrext/abs-send-time",
		"a=extmap:3 http://www.ietf.org/id/draft-holmer-rmcat-transport-wide-cc-extensions-01",
		"a=extmap:1 urn:ietf:params:rtp-hdrext:ssrc-audio-level",
		"a=recvonly",
		"a=rtcp-mux",
		"a=rtcp-rsize",
		"a=rtpmap:111 opus/48000/2",
		"a=rtcp-fb:111 transport-cc",
		"a=rtcp-fb:111 nack",
		"a=fmtp:111 stereo=1;usedtx=1;useinbandfec=1",
		fmt.Sprintf("m=video %d UDP/TLS/RTP/SAVPF 99 100", port),
		fmt.Sprintf("c=IN IP4 %s", ip),
		"a=rtcp:9 IN IP4 0.0.0.0",
		udpCandidate,
		fmt.Sprintf("a=ice-ufrag:%s", iceUfrag),
		fmt.Sprintf("a=ice-pwd:%s", icePwd),
		"a=ice-options:renomination",
		fmt.Sprintf("a=fingerprint:%s %s", algorithmName, algorithmValue),
		"a=setup:active",
		fmt.Sprintf("a=mid:%s", videoMid),
		"a=extmap:4 urn:ietf:params:rtp-hdrext:sdes:mid",
		"a=extmap:10 urn:ietf:params:rtp-hdrext:sdes:rtp-stream-id",
		"a=extmap:11 urn:ietf:params:rtp-hdrext:sdes:repaired-rtp-stream-id",
		"a=extmap:2 http://www.webrtc.org/experiments/rtp-hdrext/abs-send-time",
		"a=extmap:3 http://www.ietf.org/id/draft-holmer-rmcat-transport-wide-cc-extensions-01",
		"a=extmap:13 urn:3gpp:video-orientation",
		"a=extmap:14 urn:ietf:params:rtp-hdrext:toffset",
		"a=extmap:5 http://www.webrtc.org/experiments/rtp-hdrext/playout-delay",
		"a=recvonly",
		"a=rtcp-mux",
		"a=rtcp-rsize",
		"a=rtpmap:99 H264/90000",
		"a=rtcp-fb:99 transport-cc",
		"a=rtcp-fb:99 ccm fir",
		"a=rtcp-fb:99 nack",
		"a=rtcp-fb:99 nack pli",
		"a=fmtp:99 level-asymmetry-allowed=1;packetization-mode=1;profile-level-id=42001f",
		"a=rtpmap:100 rtx/90000",
		"a=fmtp:100 apt=99",
		//"a=simple",
	}

	// Add ICE candidates
	sdpLines = append(sdpLines)

	return strings.Join(sdpLines, "\r\n") + "\r\n"
}
