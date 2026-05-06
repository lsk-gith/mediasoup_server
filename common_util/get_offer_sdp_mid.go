package common_util

import "strings"

func GetOfferSDPMid(sdp string) (audioMid, videoMid string) {
	lines := strings.Split(sdp, "\r\n")
	audioMid = "0"
	videoMid = "1"
	video := false
	for _, line := range lines {
		if strings.HasPrefix(line, "a=mid:") && !video {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) != 2 {
				return audioMid, videoMid
			}
			video = true
			audioMid = parts[1]
		}
		if strings.HasPrefix(line, "a=mid:") && video {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) != 2 {
				return audioMid, videoMid
			}
			videoMid = parts[1]
		}
	}
	return
}
