package appsyncdynamodb

import (
	"crypto/rand"
	"strings"
	"sync"
	"time"

	"github.com/holbrookab/go-temporal-ai-sdk/streaming"
)

var (
	eventIDMu     sync.Mutex
	lastEventTime int64
	lastEntropy   [10]byte
)

func newEventID() string {
	eventIDMu.Lock()
	defer eventIDMu.Unlock()

	now := time.Now().UnixMilli()
	if now > lastEventTime {
		lastEventTime = now
		if _, err := rand.Read(lastEntropy[:]); err != nil {
			clear(lastEntropy[:])
		}
	} else {
		now = lastEventTime
		incrementEntropy()
	}
	return encodeULID(now, lastEntropy)
}

func incrementEntropy() {
	for i := len(lastEntropy) - 1; i >= 0; i-- {
		lastEntropy[i]++
		if lastEntropy[i] != 0 {
			return
		}
	}
}

func encodeULID(ms int64, entropy [10]byte) string {
	const alphabet = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"
	var out [26]byte
	t := uint64(ms)
	for i := 9; i >= 0; i-- {
		out[i] = alphabet[t&0x1f]
		t >>= 5
	}
	var buf uint32
	var bits uint
	j := 10
	for _, b := range entropy {
		buf = (buf << 8) | uint32(b)
		bits += 8
		for bits >= 5 {
			bits -= 5
			out[j] = alphabet[(buf>>bits)&0x1f]
			j++
		}
	}
	return string(out[:])
}

func attemptStorageKey(input streaming.AttemptRef) string {
	toolCallID := input.ToolCallID
	if toolCallID == "" {
		toolCallID = "_"
	}
	return input.StreamID + "#" + string(input.Lane) + "#" + toolCallID + "#" + input.AttemptID
}

func trimSlashes(value string) string {
	return strings.Trim(value, "/")
}
