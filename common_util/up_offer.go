package common_util

import (
	"fmt"
	"mediasoup_server/bean"
	"strings"
)

// 解析上行sdp，组装出发给worker的参数
func ParseUpOfferSDP(sdp string) (audioParams, videoParams bean.MediaParameters, err error) {
	lines := strings.Split(sdp, "\r\n")

	var audioSsrc uint32
	var videoSsrc1 uint32
	var videoSsrc2 uint32
	simulcast := false
	for _, line := range lines {
		if strings.Contains(line, "simulcast") {
			simulcast = true
		} else {
			ssrc := uint32(0)
			if strings.HasPrefix(line, "a=ssrc:") {
				ssrcStr := strings.TrimPrefix(line, "a=ssrc:")
				if spaceIndex := strings.Index(ssrcStr, " "); spaceIndex != -1 {
					ssrcStr = ssrcStr[:spaceIndex]
				}
				fmt.Sscanf(ssrcStr, "%d", &ssrc)
				fmt.Printf("ssrc:%d\n", ssrc)
				if audioSsrc == 0 || audioSsrc == ssrc {
					audioSsrc = ssrc
				} else if videoSsrc1 == 0 || videoSsrc1 == ssrc {
					videoSsrc1 = ssrc
				} else if videoSsrc2 == 0 || videoSsrc2 == ssrc {
					videoSsrc2 = ssrc
				}
			}
		}

	}
	fmt.Printf("audioSsrc:%d videoSsrc1:%d videoSsrc2:%d\n", audioSsrc, videoSsrc1, videoSsrc2)
	audioParams = getAudioOpusParam(audioSsrc)
	if simulcast {
		videoParams = getSimulcastVideoVP8Param()
		//videoParams = getSimulcastVideoH264Param()
	} else {
		videoParams = getVideoVP8Param(videoSsrc1, videoSsrc2)
		//videoParams = getVideoH264Param(videoSsrc1, videoSsrc2)
	}
	return audioParams, videoParams, nil
}

func getAudioOpusParam(ssrc uint32) bean.MediaParameters {

	codecs := make([]bean.Codec, 0)
	rtcpFeedbacks := make([]bean.RtcpFeedback, 0)
	rtcpFeedbacks = append(rtcpFeedbacks, bean.RtcpFeedback{
		Type:      "transport-cc",
		Parameter: "",
	})
	rtcpFeedbacks = append(rtcpFeedbacks, bean.RtcpFeedback{
		Type:      "nack",
		Parameter: "",
	})
	parameters := map[string]interface{}{}
	parameters["minptime"] = 10
	parameters["useinbandfec"] = 1
	parameters["sprop-stereo"] = 1
	parameters["usedtx"] = 1
	codecs = append(codecs, bean.Codec{
		MimeType:     "audio/opus",
		PayloadType:  111,
		ClockRate:    48000,
		Channels:     2,
		Parameters:   parameters,
		RtcpFeedback: rtcpFeedbacks,
	})

	headerExtensions := make([]bean.HeaderExtension, 0)
	headerExtensions = append(headerExtensions, bean.HeaderExtension{
		Uri:     "urn:ietf:params:rtp-hdrext:sdes:mid",
		Id:      4,
		Encrypt: false,
	})

	headerExtensions = append(headerExtensions, bean.HeaderExtension{
		Uri:     "http://www.webrtc.org/experiments/rtp-hdrext/abs-send-time",
		Id:      2,
		Encrypt: false,
	})
	headerExtensions = append(headerExtensions, bean.HeaderExtension{
		Uri:     "http://www.ietf.org/id/draft-holmer-rmcat-transport-wide-cc-extensions-01",
		Id:      3,
		Encrypt: false,
	})
	headerExtensions = append(headerExtensions, bean.HeaderExtension{
		Uri:     "urn:ietf:params:rtp-hdrext:ssrc-audio-level",
		Id:      1,
		Encrypt: false,
	})

	encodings := make([]bean.Encoding, 0)
	encodings = append(encodings, bean.Encoding{
		Ssrc: ssrc,
		Dtx:  false,
	})

	rtpParameters := bean.RtpParameters{
		Mid:              "0",
		Codecs:           codecs,
		HeaderExtensions: headerExtensions,
		Encodings:        encodings,
		Rtcp: bean.Rtcp{
			Cname:       "audio",
			ReducedSize: true,
		},
	}

	codecsMap := make([]bean.CodecMapping, 0)
	codecsMap = append(codecsMap, bean.CodecMapping{
		PayloadType:       111,
		MappedPayloadType: 100,
	})

	encodingsMap := make([]bean.EncodingMapping, 0)
	encodingsMap = append(encodingsMap, bean.EncodingMapping{
		MappedSsrc: ssrc + 1,
		Ssrc:       ssrc,
	})
	rtpMapping := bean.RtpMapping{
		Codecs:    codecsMap,
		Encodings: encodingsMap,
	}

	audioParams := bean.MediaParameters{
		Kind:          "audio",
		Paused:        false,
		RtpParameters: rtpParameters,
		RtpMapping:    rtpMapping,
	}
	return audioParams

}

func getVideoVP8Param(ssrc1, ssrc2 uint32) bean.MediaParameters {
	codecs := make([]bean.Codec, 0)
	rtcpFeedbacks := make([]bean.RtcpFeedback, 0)
	rtcpFeedbacks = append(rtcpFeedbacks, bean.RtcpFeedback{
		Type:      "goog-remb",
		Parameter: "",
	})
	rtcpFeedbacks = append(rtcpFeedbacks, bean.RtcpFeedback{
		Type:      "transport-cc",
		Parameter: "",
	})
	rtcpFeedbacks = append(rtcpFeedbacks, bean.RtcpFeedback{
		Type:      "ccm",
		Parameter: "fir",
	})
	rtcpFeedbacks = append(rtcpFeedbacks, bean.RtcpFeedback{
		Type:      "nack",
		Parameter: "",
	})
	rtcpFeedbacks = append(rtcpFeedbacks, bean.RtcpFeedback{
		Type:      "nack",
		Parameter: "pli",
	})
	codecs = append(codecs, bean.Codec{
		MimeType:     "video/VP8",
		PayloadType:  96,
		ClockRate:    90000,
		RtcpFeedback: rtcpFeedbacks,
	})
	parameters := map[string]interface{}{}
	parameters["apt"] = 96
	codecs = append(codecs, bean.Codec{
		MimeType:    "video/rtx",
		PayloadType: 97,
		ClockRate:   90000,
		Parameters:  parameters,
	})

	headerExtensions := make([]bean.HeaderExtension, 0)
	headerExtensions = append(headerExtensions, bean.HeaderExtension{
		Uri:     "urn:ietf:params:rtp-hdrext:sdes:mid",
		Id:      4,
		Encrypt: false,
	})
	headerExtensions = append(headerExtensions, bean.HeaderExtension{
		Uri:     "urn:ietf:params:rtp-hdrext:sdes:rtp-stream-id",
		Id:      10,
		Encrypt: false,
	})
	headerExtensions = append(headerExtensions, bean.HeaderExtension{
		Uri:     "urn:ietf:params:rtp-hdrext:sdes:repaired-rtp-stream-id",
		Id:      11,
		Encrypt: false,
	})
	headerExtensions = append(headerExtensions, bean.HeaderExtension{
		Uri:     "http://www.webrtc.org/experiments/rtp-hdrext/abs-send-time",
		Id:      2,
		Encrypt: false,
	})
	headerExtensions = append(headerExtensions, bean.HeaderExtension{
		Uri:     "http://www.ietf.org/id/draft-holmer-rmcat-transport-wide-cc-extensions-01",
		Id:      3,
		Encrypt: false,
	})
	headerExtensions = append(headerExtensions, bean.HeaderExtension{
		Uri:     "urn:3gpp:video-orientation",
		Id:      13,
		Encrypt: false,
	})
	headerExtensions = append(headerExtensions, bean.HeaderExtension{
		Uri:     "urn:ietf:params:rtp-hdrext:toffset",
		Id:      14,
		Encrypt: false,
	})

	encodings := make([]bean.Encoding, 0)
	rtx := make(map[string]uint32)
	rtx["ssrc"] = ssrc2
	encodings = append(encodings, bean.Encoding{
		Ssrc: ssrc1,
		Rtx:  rtx,
		Dtx:  false,
	})

	rtpParameters := bean.RtpParameters{
		Mid:              "1",
		Codecs:           codecs,
		HeaderExtensions: headerExtensions,
		Encodings:        encodings,
		Rtcp: bean.Rtcp{
			Cname:       "video-01",
			ReducedSize: true,
		},
	}

	codecsMap := make([]bean.CodecMapping, 0)
	codecsMap = append(codecsMap, bean.CodecMapping{
		PayloadType:       96,
		MappedPayloadType: 101,
	})
	codecsMap = append(codecsMap, bean.CodecMapping{
		PayloadType:       97,
		MappedPayloadType: 102,
	})
	encodingsMap := make([]bean.EncodingMapping, 0)
	encodingsMap = append(encodingsMap, bean.EncodingMapping{
		MappedSsrc: ssrc1 + 1,
		Ssrc:       ssrc1,
	})
	rtpMapping := bean.RtpMapping{
		Codecs:    codecsMap,
		Encodings: encodingsMap,
	}

	audioParams := bean.MediaParameters{
		Kind:          "video",
		Paused:        false,
		RtpParameters: rtpParameters,
		RtpMapping:    rtpMapping,
	}

	return audioParams

}

func getVideoH264Param(ssrc1, ssrc2 uint32) bean.MediaParameters {
	codecs := make([]bean.Codec, 0)
	rtcpFeedbacks := make([]bean.RtcpFeedback, 0)
	rtcpFeedbacks = append(rtcpFeedbacks, bean.RtcpFeedback{
		Type:      "goog-remb",
		Parameter: "",
	})
	rtcpFeedbacks = append(rtcpFeedbacks, bean.RtcpFeedback{
		Type:      "transport-cc",
		Parameter: "",
	})
	rtcpFeedbacks = append(rtcpFeedbacks, bean.RtcpFeedback{
		Type:      "ccm",
		Parameter: "fir",
	})
	rtcpFeedbacks = append(rtcpFeedbacks, bean.RtcpFeedback{
		Type:      "nack",
		Parameter: "",
	})
	rtcpFeedbacks = append(rtcpFeedbacks, bean.RtcpFeedback{
		Type:      "nack",
		Parameter: "pli",
	})
	firstParameters := map[string]interface{}{}
	firstParameters["level-asymmetry-allowed"] = 1
	firstParameters["packetization-mode"] = 1
	firstParameters["profile-level-id"] = "640c1f"
	codecs = append(codecs, bean.Codec{
		MimeType:     "video/H264",
		PayloadType:  96,
		ClockRate:    90000,
		RtcpFeedback: rtcpFeedbacks,
		Parameters:   firstParameters,
	})
	parameters := map[string]interface{}{}
	parameters["apt"] = 96
	codecs = append(codecs, bean.Codec{
		MimeType:    "video/rtx",
		PayloadType: 97,
		ClockRate:   90000,
		Parameters:  parameters,
	})

	headerExtensions := make([]bean.HeaderExtension, 0)
	headerExtensions = append(headerExtensions, bean.HeaderExtension{
		Uri:     "urn:ietf:params:rtp-hdrext:sdes:mid",
		Id:      4,
		Encrypt: false,
	})
	headerExtensions = append(headerExtensions, bean.HeaderExtension{
		Uri:     "urn:ietf:params:rtp-hdrext:sdes:rtp-stream-id",
		Id:      10,
		Encrypt: false,
	})
	headerExtensions = append(headerExtensions, bean.HeaderExtension{
		Uri:     "urn:ietf:params:rtp-hdrext:sdes:repaired-rtp-stream-id",
		Id:      11,
		Encrypt: false,
	})
	headerExtensions = append(headerExtensions, bean.HeaderExtension{
		Uri:     "http://www.webrtc.org/experiments/rtp-hdrext/abs-send-time",
		Id:      2,
		Encrypt: false,
	})
	headerExtensions = append(headerExtensions, bean.HeaderExtension{
		Uri:     "http://www.ietf.org/id/draft-holmer-rmcat-transport-wide-cc-extensions-01",
		Id:      3,
		Encrypt: false,
	})
	headerExtensions = append(headerExtensions, bean.HeaderExtension{
		Uri:     "urn:3gpp:video-orientation",
		Id:      13,
		Encrypt: false,
	})
	headerExtensions = append(headerExtensions, bean.HeaderExtension{
		Uri:     "urn:ietf:params:rtp-hdrext:toffset",
		Id:      14,
		Encrypt: false,
	})

	encodings := make([]bean.Encoding, 0)
	rtx := make(map[string]uint32)
	rtx["ssrc"] = ssrc2
	encodings = append(encodings, bean.Encoding{
		Ssrc: ssrc1,
		Rtx:  rtx,
		Dtx:  false,
	})

	rtpParameters := bean.RtpParameters{
		Mid:              "1",
		Codecs:           codecs,
		HeaderExtensions: headerExtensions,
		Encodings:        encodings,
		Rtcp: bean.Rtcp{
			Cname:       "video-01",
			ReducedSize: true,
		},
	}

	codecsMap := make([]bean.CodecMapping, 0)
	codecsMap = append(codecsMap, bean.CodecMapping{
		PayloadType:       96,
		MappedPayloadType: 101,
	})
	codecsMap = append(codecsMap, bean.CodecMapping{
		PayloadType:       97,
		MappedPayloadType: 102,
	})
	encodingsMap := make([]bean.EncodingMapping, 0)
	encodingsMap = append(encodingsMap, bean.EncodingMapping{
		MappedSsrc: ssrc1 + 1,
		Ssrc:       ssrc1,
	})
	rtpMapping := bean.RtpMapping{
		Codecs:    codecsMap,
		Encodings: encodingsMap,
	}

	audioParams := bean.MediaParameters{
		Kind:          "video",
		Paused:        false,
		RtpParameters: rtpParameters,
		RtpMapping:    rtpMapping,
	}

	return audioParams

}

func getSimulcastVideoVP8Param() bean.MediaParameters {
	codecs := make([]bean.Codec, 0)
	rtcpFeedbacks := make([]bean.RtcpFeedback, 0)
	rtcpFeedbacks = append(rtcpFeedbacks, bean.RtcpFeedback{
		Type:      "goog-remb",
		Parameter: "",
	})
	rtcpFeedbacks = append(rtcpFeedbacks, bean.RtcpFeedback{
		Type:      "transport-cc",
		Parameter: "",
	})
	rtcpFeedbacks = append(rtcpFeedbacks, bean.RtcpFeedback{
		Type:      "ccm",
		Parameter: "fir",
	})
	rtcpFeedbacks = append(rtcpFeedbacks, bean.RtcpFeedback{
		Type:      "nack",
		Parameter: "",
	})
	rtcpFeedbacks = append(rtcpFeedbacks, bean.RtcpFeedback{
		Type:      "nack",
		Parameter: "pli",
	})
	codecs = append(codecs, bean.Codec{
		MimeType:     "video/VP8",
		PayloadType:  96,
		ClockRate:    90000,
		RtcpFeedback: rtcpFeedbacks,
	})
	parameters := map[string]interface{}{}
	parameters["apt"] = 96
	codecs = append(codecs, bean.Codec{
		MimeType:    "video/rtx",
		PayloadType: 97,
		ClockRate:   90000,
		Parameters:  parameters,
	})

	headerExtensions := make([]bean.HeaderExtension, 0)
	headerExtensions = append(headerExtensions, bean.HeaderExtension{
		Uri:     "urn:ietf:params:rtp-hdrext:sdes:mid",
		Id:      4,
		Encrypt: false,
	})
	headerExtensions = append(headerExtensions, bean.HeaderExtension{
		Uri:     "urn:ietf:params:rtp-hdrext:sdes:rtp-stream-id",
		Id:      10,
		Encrypt: false,
	})
	headerExtensions = append(headerExtensions, bean.HeaderExtension{
		Uri:     "urn:ietf:params:rtp-hdrext:sdes:repaired-rtp-stream-id",
		Id:      11,
		Encrypt: false,
	})
	headerExtensions = append(headerExtensions, bean.HeaderExtension{
		Uri:     "http://www.webrtc.org/experiments/rtp-hdrext/abs-send-time",
		Id:      2,
		Encrypt: false,
	})
	headerExtensions = append(headerExtensions, bean.HeaderExtension{
		Uri:     "http://www.ietf.org/id/draft-holmer-rmcat-transport-wide-cc-extensions-01",
		Id:      3,
		Encrypt: false,
	})
	headerExtensions = append(headerExtensions, bean.HeaderExtension{
		Uri:     "urn:3gpp:video-orientation",
		Id:      13,
		Encrypt: false,
	})
	headerExtensions = append(headerExtensions, bean.HeaderExtension{
		Uri:     "urn:ietf:params:rtp-hdrext:toffset",
		Id:      14,
		Encrypt: false,
	})

	encodings := make([]bean.Encoding, 0)
	encodings = append(encodings, bean.Encoding{
		Active:                true,
		ScalabilityMode:       "L1T3",
		ScaleResolutionDownBy: 4,
		MaxBitrate:            500000,
		Rid:                   "r0",
		Dtx:                   false,
	})

	encodings = append(encodings, bean.Encoding{
		Active:                true,
		ScalabilityMode:       "L1T3",
		ScaleResolutionDownBy: 2,
		MaxBitrate:            1000000,
		Rid:                   "r1",
		Dtx:                   false,
	})
	encodings = append(encodings, bean.Encoding{
		Active:                true,
		ScalabilityMode:       "L1T3",
		ScaleResolutionDownBy: 1,
		MaxBitrate:            5000000,
		Rid:                   "r2",
		Dtx:                   false,
	})
	rtpParameters := bean.RtpParameters{
		Mid:              "1",
		Codecs:           codecs,
		HeaderExtensions: headerExtensions,
		Encodings:        encodings,
		Rtcp: bean.Rtcp{
			Cname:       "video-01",
			ReducedSize: true,
		},
	}

	codecsMap := make([]bean.CodecMapping, 0)
	codecsMap = append(codecsMap, bean.CodecMapping{
		PayloadType:       96,
		MappedPayloadType: 101,
	})
	codecsMap = append(codecsMap, bean.CodecMapping{
		PayloadType:       97,
		MappedPayloadType: 102,
	})
	encodingsMap := make([]bean.EncodingMapping, 0)
	ssrc := GenerateSsrc()
	encodingsMap = append(encodingsMap, bean.EncodingMapping{
		MappedSsrc:      ssrc,
		Rid:             "r0",
		ScalabilityMode: "L1T3",
	})
	encodingsMap = append(encodingsMap, bean.EncodingMapping{
		MappedSsrc:      ssrc + 1,
		Rid:             "r1",
		ScalabilityMode: "L1T3",
	})
	encodingsMap = append(encodingsMap, bean.EncodingMapping{
		MappedSsrc:      ssrc + 2,
		Rid:             "r2",
		ScalabilityMode: "L1T3",
	})
	rtpMapping := bean.RtpMapping{
		Codecs:    codecsMap,
		Encodings: encodingsMap,
	}

	audioParams := bean.MediaParameters{
		Kind:          "video",
		Paused:        false,
		RtpParameters: rtpParameters,
		RtpMapping:    rtpMapping,
	}

	return audioParams
}
func getSimulcastVideoH264Param() bean.MediaParameters {
	codecs := make([]bean.Codec, 0)
	rtcpFeedbacks := make([]bean.RtcpFeedback, 0)
	rtcpFeedbacks = append(rtcpFeedbacks, bean.RtcpFeedback{
		Type:      "goog-remb",
		Parameter: "",
	})
	rtcpFeedbacks = append(rtcpFeedbacks, bean.RtcpFeedback{
		Type:      "transport-cc",
		Parameter: "",
	})
	rtcpFeedbacks = append(rtcpFeedbacks, bean.RtcpFeedback{
		Type:      "ccm",
		Parameter: "fir",
	})
	rtcpFeedbacks = append(rtcpFeedbacks, bean.RtcpFeedback{
		Type:      "nack",
		Parameter: "",
	})
	rtcpFeedbacks = append(rtcpFeedbacks, bean.RtcpFeedback{
		Type:      "nack",
		Parameter: "pli",
	})
	codecs = append(codecs, bean.Codec{
		MimeType:     "video/H264",
		PayloadType:  96,
		ClockRate:    90000,
		RtcpFeedback: rtcpFeedbacks,
	})
	parameters := map[string]interface{}{}
	parameters["apt"] = 96
	codecs = append(codecs, bean.Codec{
		MimeType:    "video/rtx",
		PayloadType: 97,
		ClockRate:   90000,
		Parameters:  parameters,
	})

	headerExtensions := make([]bean.HeaderExtension, 0)
	headerExtensions = append(headerExtensions, bean.HeaderExtension{
		Uri:     "urn:ietf:params:rtp-hdrext:sdes:mid",
		Id:      4,
		Encrypt: false,
	})
	headerExtensions = append(headerExtensions, bean.HeaderExtension{
		Uri:     "urn:ietf:params:rtp-hdrext:sdes:rtp-stream-id",
		Id:      10,
		Encrypt: false,
	})
	headerExtensions = append(headerExtensions, bean.HeaderExtension{
		Uri:     "urn:ietf:params:rtp-hdrext:sdes:repaired-rtp-stream-id",
		Id:      11,
		Encrypt: false,
	})
	headerExtensions = append(headerExtensions, bean.HeaderExtension{
		Uri:     "http://www.webrtc.org/experiments/rtp-hdrext/abs-send-time",
		Id:      2,
		Encrypt: false,
	})
	headerExtensions = append(headerExtensions, bean.HeaderExtension{
		Uri:     "http://www.ietf.org/id/draft-holmer-rmcat-transport-wide-cc-extensions-01",
		Id:      3,
		Encrypt: false,
	})
	headerExtensions = append(headerExtensions, bean.HeaderExtension{
		Uri:     "urn:3gpp:video-orientation",
		Id:      13,
		Encrypt: false,
	})
	headerExtensions = append(headerExtensions, bean.HeaderExtension{
		Uri:     "urn:ietf:params:rtp-hdrext:toffset",
		Id:      14,
		Encrypt: false,
	})

	encodings := make([]bean.Encoding, 0)
	encodings = append(encodings, bean.Encoding{
		Active:                true,
		ScalabilityMode:       "L1T3",
		ScaleResolutionDownBy: 4,
		MaxBitrate:            500000,
		Rid:                   "r0",
		Dtx:                   false,
	})

	encodings = append(encodings, bean.Encoding{
		Active:                true,
		ScalabilityMode:       "L1T3",
		ScaleResolutionDownBy: 2,
		MaxBitrate:            1000000,
		Rid:                   "r1",
		Dtx:                   false,
	})
	encodings = append(encodings, bean.Encoding{
		Active:                true,
		ScalabilityMode:       "L1T3",
		ScaleResolutionDownBy: 1,
		MaxBitrate:            5000000,
		Rid:                   "r2",
		Dtx:                   false,
	})
	rtpParameters := bean.RtpParameters{
		Mid:              "1",
		Codecs:           codecs,
		HeaderExtensions: headerExtensions,
		Encodings:        encodings,
		Rtcp: bean.Rtcp{
			Cname:       "video-01",
			ReducedSize: true,
		},
	}

	codecsMap := make([]bean.CodecMapping, 0)
	codecsMap = append(codecsMap, bean.CodecMapping{
		PayloadType:       96,
		MappedPayloadType: 101,
	})
	codecsMap = append(codecsMap, bean.CodecMapping{
		PayloadType:       97,
		MappedPayloadType: 102,
	})
	encodingsMap := make([]bean.EncodingMapping, 0)
	ssrc := GenerateSsrc()
	encodingsMap = append(encodingsMap, bean.EncodingMapping{
		MappedSsrc:      ssrc,
		Rid:             "r0",
		ScalabilityMode: "L1T3",
	})
	encodingsMap = append(encodingsMap, bean.EncodingMapping{
		MappedSsrc:      ssrc + 1,
		Rid:             "r1",
		ScalabilityMode: "L1T3",
	})
	encodingsMap = append(encodingsMap, bean.EncodingMapping{
		MappedSsrc:      ssrc + 2,
		Rid:             "r2",
		ScalabilityMode: "L1T3",
	})
	rtpMapping := bean.RtpMapping{
		Codecs:    codecsMap,
		Encodings: encodingsMap,
	}

	audioParams := bean.MediaParameters{
		Kind:          "video",
		Paused:        false,
		RtpParameters: rtpParameters,
		RtpMapping:    rtpMapping,
	}

	return audioParams
}
