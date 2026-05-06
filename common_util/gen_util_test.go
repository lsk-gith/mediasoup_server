package common_util

import (
	"fmt"
	"testing"
)

func TestGenerateSsrc(t *testing.T) {
	fmt.Printf("first:%d\n", GenerateSsrc())
	fmt.Printf("second:%d\n", GenerateSsrc())
}
