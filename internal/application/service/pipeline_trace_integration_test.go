package service

import (
	"encoding/json"
	"testing"

	chatpipeline "github.com/Tencent/WeKnora/internal/application/service/chat_pipeline"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/require"
)

func TestPipelineTraceCapturesExecutedStagesAndImmutableDocuments(t *testing.T) {
	manager := chatpipeline.NewEventManager()
	chatpipeline.NewPluginFilterTopK(manager)
	chatpipeline.NewPluginIntoChatMessage(manager, nil, nil)
	svc := &sessionService{eventManager: manager}
	steps := make([]types.PipelineTraceStep, 0)
	cm := &types.ChatManage{
		PipelineRequest: types.PipelineRequest{
			Query: "How long are backups retained?", RerankTopK: 1,
			SummaryConfig: types.SummaryConfig{ContextTemplate: "{{contexts}}\n{{query}}"},
		},
		PipelineState: types.PipelineState{MergeResult: []*types.SearchResult{
			{ID: "backup-policy", Content: "Backups are retained for seven days.", Score: 0.9},
			{ID: "other", Content: "Unrelated passage.", Score: 0.2},
		}},
		PipelineContext: types.PipelineContext{
			PipelineTraceObserver: func(step types.PipelineTraceStep) { steps = append(steps, step) },
		},
	}
	require.NoError(t, svc.KnowledgeQAByEvent(t.Context(), cm, []types.EventType{
		types.FILTER_TOP_K, types.INTO_CHAT_MESSAGE,
	}))
	require.Len(t, steps, 2)
	require.Equal(t, string(types.FILTER_TOP_K), steps[0].ID)
	var before, after pipelineTraceState
	require.NoError(t, json.Unmarshal([]byte(steps[0].Input), &before))
	require.NoError(t, json.Unmarshal([]byte(steps[0].Output), &after))
	require.Len(t, before.ContextDocuments, 2)
	require.Len(t, after.ContextDocuments, 1)
	require.Len(t, steps[0].Documents, 1)
	require.Equal(t, "backup-policy", steps[0].Documents[0].ID)
	require.NoError(t, json.Unmarshal([]byte(steps[1].Output), &after))
	require.Equal(t, cm.UserContent, after.UserContent)
	require.Contains(t, after.UserContent, cm.MergeResult[0].Content)
	cm.MergeResult[0].Score = 0.1
	cm.MergeResult[0].Content = "Changed by a later stage"
	require.Equal(t, 0.9, steps[0].Documents[0].Score)
	require.Equal(t, "Backups are retained for seven days.", steps[0].Documents[0].Content)
	require.Nil(t, cm.Clone().PipelineTraceObserver)
}

func TestPipelineTraceRecordsActualStageFailure(t *testing.T) {
	manager := chatpipeline.NewEventManager()
	chatpipeline.NewPluginIntoChatMessage(manager, nil, nil)
	svc := &sessionService{eventManager: manager}
	steps := make([]types.PipelineTraceStep, 0)
	cm := &types.ChatManage{
		PipelineRequest: types.PipelineRequest{Query: "invalid\x00query"},
		PipelineContext: types.PipelineContext{
			PipelineTraceObserver: func(step types.PipelineTraceStep) { steps = append(steps, step) },
		},
	}
	require.Error(t, svc.KnowledgeQAByEvent(t.Context(), cm, []types.EventType{types.INTO_CHAT_MESSAGE}))
	require.Len(t, steps, 1)
	require.Equal(t, "failed", steps[0].Status)
	require.NotEmpty(t, steps[0].Error)
	require.Nil(t, steps[0].Metric)
}
