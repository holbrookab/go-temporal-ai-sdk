package activities

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"unicode/utf8"
)

const (
	defaultToolArtifactMaxInlineBytes  = 24_000
	defaultToolArtifactMaxPreviewBytes = 4_000
	toolArtifactContentTypeJSON        = "application/json"
)

// ToolArtifactStore persists oversized tool inputs and outputs outside
// Temporal history. Implementations must be idempotent for repeated writes with
// the same key.
type ToolArtifactStore interface {
	PutToolArtifact(context.Context, ToolArtifactWriteInput) (*ToolArtifactRef, error)
}

// ToolArtifactPolicy controls when tool values are replaced by artifact refs.
type ToolArtifactPolicy struct {
	Enabled         bool   `json:"enabled,omitempty"`
	MaxInlineBytes  int    `json:"maxInlineBytes,omitempty"`
	MaxPreviewBytes int    `json:"maxPreviewBytes,omitempty"`
	WorkflowID      string `json:"workflowId,omitempty"`
	RunID           string `json:"runId,omitempty"`
	StoreInputs     *bool  `json:"storeInputs,omitempty"`
	StoreOutputs    *bool  `json:"storeOutputs,omitempty"`
	StoreErrors     *bool  `json:"storeErrors,omitempty"`
}

// ToolArtifactWriteInput is the external-storage write request for one raw
// value.
type ToolArtifactWriteInput struct {
	WorkflowID    string         `json:"workflowId,omitempty"`
	RunID         string         `json:"runId,omitempty"`
	ToolCallID    string         `json:"toolCallId,omitempty"`
	ToolName      string         `json:"toolName,omitempty"`
	Kind          string         `json:"kind"`
	SHA256        string         `json:"sha256"`
	ContentType   string         `json:"contentType"`
	OriginalBytes int            `json:"originalBytes"`
	Preview       any            `json:"preview,omitempty"`
	Data          []byte         `json:"-"`
	Metadata      map[string]any `json:"metadata,omitempty"`
}

// ToolArtifactRef is the compact, model-safe reference to externally stored
// tool data.
type ToolArtifactRef struct {
	ArtifactID    string `json:"artifactId"`
	Kind          string `json:"kind,omitempty"`
	OriginalBytes int    `json:"originalBytes"`
	ContentType   string `json:"contentType,omitempty"`
	SHA256        string `json:"sha256,omitempty"`
}

// ToolArtifactValue replaces an oversized tool value in model messages,
// workflow state, and lifecycle streams.
type ToolArtifactValue struct {
	ArtifactRef   *ToolArtifactRef `json:"artifactRef"`
	Preview       any              `json:"preview,omitempty"`
	OriginalBytes int              `json:"originalBytes"`
	ContentType   string           `json:"contentType,omitempty"`
	SHA256        string           `json:"sha256,omitempty"`
	Truncated     bool             `json:"truncated"`
}

func compactToolArtifacts(ctx context.Context, store ToolArtifactStore, args InvokeToolArgs, result *InvokeToolResult) (*InvokeToolResult, error) {
	if result == nil || args.Artifacts == nil || !args.Artifacts.Enabled {
		return result, nil
	}
	policy := toolArtifactPolicyWithDefaults(*args.Artifacts)
	out := *result
	var err error
	if shouldStoreToolArtifactKind(policy.StoreInputs, true) {
		out.Input, err = compactToolArtifactValue(ctx, store, policy, args, "input", out.Input)
		if err != nil {
			return nil, err
		}
	}
	if shouldStoreToolArtifactKind(policy.StoreOutputs, true) {
		out.Result, err = compactToolArtifactValue(ctx, store, policy, args, "result", out.Result)
		if err != nil {
			return nil, err
		}
		out.Output.Value, err = compactToolArtifactValue(ctx, store, policy, args, "output", out.Output.Value)
		if err != nil {
			return nil, err
		}
	}
	if out.IsError && shouldStoreToolArtifactKind(policy.StoreErrors, true) {
		out.Output.Value, err = compactToolArtifactValue(ctx, store, policy, args, "error", out.Output.Value)
		if err != nil {
			return nil, err
		}
	}
	return &out, nil
}

func compactToolArtifactValue(ctx context.Context, store ToolArtifactStore, policy ToolArtifactPolicy, args InvokeToolArgs, kind string, value any) (any, error) {
	if value == nil {
		return nil, nil
	}
	data, err := json.Marshal(value)
	if err != nil {
		data = []byte(fmt.Sprintf("%v", value))
	}
	if len(data) <= policy.MaxInlineBytes {
		return value, nil
	}
	if store == nil {
		return nil, fmt.Errorf("tool artifact store is required for %s payload of %d bytes", kind, len(data))
	}
	hash := sha256.Sum256(data)
	sha := hex.EncodeToString(hash[:])
	preview := previewJSONValue(data, policy.MaxPreviewBytes)
	ref, err := store.PutToolArtifact(ctx, ToolArtifactWriteInput{
		WorkflowID:    policy.WorkflowID,
		RunID:         policy.RunID,
		ToolCallID:    args.ToolCallID,
		ToolName:      args.ToolName,
		Kind:          kind,
		SHA256:        sha,
		ContentType:   toolArtifactContentTypeJSON,
		OriginalBytes: len(data),
		Preview:       preview,
		Data:          data,
		Metadata: map[string]any{
			"workflowId": policy.WorkflowID,
			"runId":      policy.RunID,
			"toolCallId": args.ToolCallID,
			"toolName":   args.ToolName,
			"kind":       kind,
		},
	})
	if err != nil {
		return nil, err
	}
	if ref == nil {
		return nil, fmt.Errorf("tool artifact store returned nil ref for %s payload", kind)
	}
	if ref.Kind == "" {
		ref.Kind = kind
	}
	if ref.OriginalBytes == 0 {
		ref.OriginalBytes = len(data)
	}
	if ref.ContentType == "" {
		ref.ContentType = toolArtifactContentTypeJSON
	}
	if ref.SHA256 == "" {
		ref.SHA256 = sha
	}
	return ToolArtifactValue{
		ArtifactRef:   ref,
		Preview:       preview,
		OriginalBytes: ref.OriginalBytes,
		ContentType:   ref.ContentType,
		SHA256:        ref.SHA256,
		Truncated:     true,
	}, nil
}

func toolArtifactPolicyWithDefaults(policy ToolArtifactPolicy) ToolArtifactPolicy {
	if policy.MaxInlineBytes <= 0 {
		policy.MaxInlineBytes = defaultToolArtifactMaxInlineBytes
	}
	if policy.MaxPreviewBytes <= 0 {
		policy.MaxPreviewBytes = defaultToolArtifactMaxPreviewBytes
	}
	return policy
}

func shouldStoreToolArtifactKind(value *bool, fallback bool) bool {
	if value == nil {
		return fallback
	}
	return *value
}

func previewJSONValue(data []byte, maxBytes int) any {
	if maxBytes <= 0 || len(data) <= maxBytes {
		return string(data)
	}
	return truncateUTF8Bytes(string(data), maxBytes)
}

func truncateUTF8Bytes(value string, maxBytes int) string {
	if maxBytes <= 0 || len(value) <= maxBytes {
		return value
	}
	out := value[:maxBytes]
	for !utf8.ValidString(out) && len(out) > 0 {
		out = out[:len(out)-1]
	}
	return out + "...[truncated]"
}

func artifactJSONByteLength(value any) int {
	bytes, err := json.Marshal(value)
	if err != nil {
		return len(strconv.Quote(fmt.Sprintf("%v", value)))
	}
	return len(bytes)
}
