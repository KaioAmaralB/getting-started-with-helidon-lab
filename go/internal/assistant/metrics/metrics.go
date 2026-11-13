package metrics

import "sync/atomic"

type Counters struct {
	Requests     atomic.Int64
	Responses    atomic.Int64
	Errors       atomic.Int64
	InputTokens  atomic.Int64
	OutputTokens atomic.Int64
	MissingUsage atomic.Int64
	ToolCalls    atomic.Int64
}

type Snapshot struct {
	ModelRequests                 int64 `json:"modelRequests"`
	ModelResponses                int64 `json:"modelResponses"`
	ModelErrors                   int64 `json:"modelErrors"`
	InputTokens                   int64 `json:"inputTokens"`
	OutputTokens                  int64 `json:"outputTokens"`
	ResponsesWithoutCompleteUsage int64 `json:"responsesWithoutCompleteUsage"`
	RequestedToolCalls            int64 `json:"requestedToolCalls"`
}

func (c *Counters) Snapshot() Snapshot {
	return Snapshot{c.Requests.Load(), c.Responses.Load(), c.Errors.Load(), c.InputTokens.Load(), c.OutputTokens.Load(), c.MissingUsage.Load(), c.ToolCalls.Load()}
}
