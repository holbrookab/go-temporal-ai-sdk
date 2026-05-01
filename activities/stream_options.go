package activities

import (
	"encoding/json"

	"github.com/holbrookab/go-ai/packages/ai"
	"github.com/holbrookab/go-temporal-ai-sdk/streaming"
)

const ProviderOptionsKey = "temporal"

func extractStreamOptions(options ai.LanguageModelCallOptions) (ai.LanguageModelCallOptions, streaming.Options) {
	providerOptions := cloneProviderOptions(options.ProviderOptions)
	streamOptions := parseStreamOptions(providerOptions[ProviderOptionsKey])
	delete(providerOptions, ProviderOptionsKey)
	options.ProviderOptions = providerOptions
	return options, streamOptions
}

func parseStreamOptions(value any) streaming.Options {
	if value == nil {
		return streaming.Options{}
	}
	if opts, ok := value.(streaming.Options); ok {
		return opts
	}
	bytes, err := json.Marshal(value)
	if err != nil {
		return streaming.Options{}
	}
	var opts streaming.Options
	if err := json.Unmarshal(bytes, &opts); err != nil {
		return streaming.Options{}
	}
	return opts
}

func cloneProviderOptions(options ai.ProviderOptions) ai.ProviderOptions {
	if len(options) == 0 {
		return nil
	}
	out := ai.ProviderOptions{}
	for key, value := range options {
		out[key] = value
	}
	return out
}
