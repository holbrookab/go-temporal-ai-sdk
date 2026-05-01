package streaming

import "github.com/holbrookab/go-ai/packages/ai"

func StreamFromParts(parts []ai.StreamPart) <-chan ai.StreamPart {
	out := make(chan ai.StreamPart, len(parts))
	for _, part := range parts {
		out <- part
	}
	close(out)
	return out
}
