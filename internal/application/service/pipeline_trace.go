package service

import (
	"encoding/json"
	"fmt"
	"time"

	chatpipeline "github.com/Tencent/WeKnora/internal/application/service/chat_pipeline"
	"github.com/Tencent/WeKnora/internal/types"
)

// pipelineTraceState 只记录阶段的业务输入输出，不包含模型凭据或供应商内部推理。
// pipelineTraceState captures stage data without model credentials or private provider reasoning.
type pipelineTraceState struct {
	Query            string                        `json:"query"`
	RewrittenQuery   string                        `json:"rewritten_query,omitempty"`
	Intent           types.QueryIntent             `json:"intent,omitempty"`
	SearchDocuments  []types.PipelineTraceDocument `json:"search_documents,omitempty"`
	RerankDocuments  []types.PipelineTraceDocument `json:"rerank_documents,omitempty"`
	ContextDocuments []types.PipelineTraceDocument `json:"context_documents,omitempty"`
	RenderedContexts string                        `json:"rendered_contexts,omitempty"`
	UserContent      string                        `json:"user_content,omitempty"`
	Answer           string                        `json:"answer,omitempty"`
	Usage            *types.TokenUsage             `json:"usage,omitempty"`
	Rerank           *types.RerankDiagnostics      `json:"rerank,omitempty"`
}

func snapshotPipelineDocuments(results []*types.SearchResult) []types.PipelineTraceDocument {
	documents := make([]types.PipelineTraceDocument, 0, len(results))
	for _, result := range results {
		if result != nil {
			documents = append(documents, types.PipelineTraceDocument{
				ID: result.ID, Content: result.Content, Score: result.Score, Source: result.KnowledgeTitle,
			})
		}
	}
	return documents
}

func snapshotPipelineState(cm *types.ChatManage) (string, error) {
	state := pipelineTraceState{
		Query: cm.Query, RewrittenQuery: cm.RewriteQuery, Intent: cm.Intent,
		SearchDocuments:  snapshotPipelineDocuments(cm.SearchResult),
		RerankDocuments:  snapshotPipelineDocuments(cm.RerankResult),
		ContextDocuments: snapshotPipelineDocuments(cm.MergeResult),
		RenderedContexts: cm.RenderedContexts, UserContent: cm.UserContent, Rerank: cm.RerankDiagnostics,
	}
	if cm.ChatResponse != nil {
		state.Answer = cm.ChatResponse.Content
		usage := cm.ChatResponse.Usage
		state.Usage = &usage
	}
	encoded, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return "", fmt.Errorf("encode pipeline trace snapshot: %w", err)
	}
	return string(encoded), nil
}

func completedPipelineStep(
	stage types.EventType, started time.Time, input string, cm *types.ChatManage, stageErr *chatpipeline.PluginError,
) (types.PipelineTraceStep, error) {
	output, err := snapshotPipelineState(cm)
	if err != nil {
		return types.PipelineTraceStep{}, fmt.Errorf("capture stage %s output: %w", stage, err)
	}
	var documents []*types.SearchResult
	switch stage {
	case types.CHUNK_SEARCH, types.CHUNK_SEARCH_PARALLEL, types.ENTITY_SEARCH:
		documents = cm.SearchResult
	case types.CHUNK_RERANK:
		documents = cm.RerankResult
	case types.CHUNK_MERGE, types.FILTER_TOP_K, types.INTO_CHAT_MESSAGE:
		documents = cm.MergeResult
	}
	step := types.PipelineTraceStep{
		ID: string(stage), Name: string(stage), Status: "success", DurationMS: time.Since(started).Milliseconds(),
		Input: input, Output: output, Documents: snapshotPipelineDocuments(documents),
	}
	if stageErr != nil {
		step.Status = "failed"
		step.Error = stageErr.Description
		if stageErr.Err != nil {
			step.Error += ": " + stageErr.Err.Error()
		}
	}
	return step, nil
}
