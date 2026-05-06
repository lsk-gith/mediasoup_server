package common_util

import (
	"fmt"
	"strings"
	"testing"
)

func TestGetOfferSDPMid(t *testing.T) {
	parts := strings.SplitN("a=mid:audio", ":", 2)
	if len(parts) == 2 {
		fmt.Println(parts[1])
	}
}
