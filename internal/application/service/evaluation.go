package service

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"sync"
	"time"

	"github.com/Tencent/WeKnora/internal/config"
	apperrors "github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/Tencent/WeKnora/internal/utils"
	"golang.org/x/sync/errgroup"
)

// EvaluationService 执行评测并保存可追溯的快照 / Executes evaluations and persists auditable snapshots.
type EvaluationService struct {
	config               *config.Config
	dataset              interfaces.DatasetService
	knowledgeBaseService interfaces.KnowledgeBaseService
	knowledgeService     interfaces.KnowledgeService
	sessionService       interfaces.SessionService
	modelService         interfaces.ModelService
	repository           interfaces.EvaluationRepository
	updateMu             sync.Mutex
}

func NewEvaluationService(
	config *config.Config,
	dataset interfaces.DatasetService,
	knowledgeBaseService interfaces.KnowledgeBaseService,
	knowledgeService interfaces.KnowledgeService,
	sessionService interfaces.SessionService,
	modelService interfaces.ModelService,
	repository interfaces.EvaluationRepository,
) interfaces.EvaluationService {
	return &EvaluationService{
		config: config, dataset: dataset, knowledgeBaseService: knowledgeBaseService,
		knowledgeService: knowledgeService, sessionService: sessionService,
		modelService: modelService, repository: repository,
	}
}

// authorizeEvaluationWorkspace 拒绝知识库受限密钥访问空间级评测 / Rejects KB-restricted keys for workspace-wide evaluation.
func authorizeEvaluationWorkspace(ctx context.Context) error {
	if scope, ok := types.TenantAPIKeyScopeFromContext(ctx); ok && scope.IsKnowledgeBaseRestricted() {
		return apperrors.NewForbiddenError("Workspace-wide evaluation requires an API key without knowledge-base restrictions")
	}
	return nil
}

func (e *EvaluationService) EvaluationResult(ctx context.Context, taskID string) (*types.EvaluationDetail, error) {
	if err := authorizeEvaluationWorkspace(ctx); err != nil {
		return nil, err
	}
	return e.repository.Get(ctx, types.MustTenantIDFromContext(ctx), taskID)
}

func (e *EvaluationService) ListEvaluationRuns(ctx context.Context, page int, pageSize int) (*types.EvaluationRunPage, error) {
	if err := authorizeEvaluationWorkspace(ctx); err != nil {
		return nil, err
	}
	return e.repository.List(ctx, types.MustTenantIDFromContext(ctx), page, pageSize)
}

func (e *EvaluationService) GetEvaluationDataset(ctx context.Context, datasetID string) (*types.EvaluationDataset, error) {
	if err := authorizeEvaluationWorkspace(ctx); err != nil {
		return nil, err
	}
	return e.dataset.GetEvaluationDataset(ctx, datasetID)
}

func (e *EvaluationService) ListEvaluationDatasets(ctx context.Context) ([]types.EvaluationDatasetSummary, error) {
	return e.dataset.ListEvaluationDatasets(ctx)
}

func (e *EvaluationService) GetEvaluationDatasetVersion(ctx context.Context, datasetID string, version string) (*types.EvaluationDataset, error) {
	return e.dataset.GetEvaluationDatasetVersion(ctx, datasetID, version)
}

// Evaluation 在异步执行前固定题目及参数 / Freezes questions and configuration before asynchronous execution.
func (e *EvaluationService) Evaluation(ctx context.Context, datasetID string, datasetVersion string, knowledgeBaseID string, chatModelID string, rerankModelID string) (*types.EvaluationDetail, error) {
	if err := authorizeEvaluationWorkspace(ctx); err != nil {
		return nil, err
	}
	tenantID := types.MustTenantIDFromContext(ctx)
	if datasetID == "" {
		datasetID = "default"
	}
	snapshot, err := e.dataset.GetEvaluationDatasetVersion(ctx, datasetID, datasetVersion)
	if err != nil {
		return nil, err
	}
	for _, item := range snapshot.Items {
		if item.Answer == "" || len(item.PIDs) == 0 {
			return nil, apperrors.NewBadRequestError(fmt.Sprintf("Question %d requires a reference answer and evidence passages; abstention scoring is not supported", item.QID))
		}
	}
	models, err := e.modelService.ListModels(ctx)
	if err != nil {
		return nil, fmt.Errorf("list models for evaluation: %w", err)
	}
	chatModelID, err = resolveEvaluationModel(models, types.ModelTypeKnowledgeQA, chatModelID)
	if err != nil {
		return nil, err
	}
	if chatModelID == "" {
		return nil, apperrors.NewBadRequestError("Evaluation requires a configured chat model")
	}
	rerankModelID, err = resolveEvaluationModel(models, types.ModelTypeRerank, rerankModelID)
	if err != nil {
		return nil, err
	}

	kb := &types.KnowledgeBase{Name: "Evaluation", Description: "Evaluation dataset corpus", IsTemporary: true}
	if knowledgeBaseID != "" {
		reference, err := e.knowledgeBaseService.GetKnowledgeBaseByID(ctx, knowledgeBaseID)
		if err != nil {
			return nil, err
		}
		if reference.TenantID != tenantID {
			return nil, apperrors.NewNotFoundError("Reference knowledge base not found in this workspace")
		}
		kb.EmbeddingModelID = reference.EmbeddingModelID
		kb.SummaryModelID = reference.SummaryModelID
		kb.ChunkingConfig = reference.ChunkingConfig
		kb.IndexingStrategy = reference.IndexingStrategy
		kb.VectorStoreID = reference.VectorStoreID
	} else {
		kb.EmbeddingModelID, err = resolveEvaluationModel(models, types.ModelTypeEmbedding, "")
		if err != nil {
			return nil, err
		}
		if kb.EmbeddingModelID == "" {
			return nil, apperrors.NewBadRequestError("Evaluation requires a configured embedding model")
		}
		kb.SummaryModelID = chatModelID
	}
	createdKB, err := e.knowledgeBaseService.CreateKnowledgeBase(ctx, kb)
	if err != nil {
		return nil, fmt.Errorf("create evaluation knowledge base: %w", err)
	}
	detail := &types.EvaluationDetail{
		Task: &types.EvaluationTask{
			ID: utils.GenerateTaskID("evaluation", tenantID, datasetID), TenantID: tenantID,
			DatasetID: datasetID, DatasetVersion: snapshot.Version, DatasetVersionLabel: snapshot.PublishedVersion, KnowledgeBaseID: knowledgeBaseID,
			MetricVersion: "corpus-passage-v3", EmbeddingModelID: createdKB.EmbeddingModelID,
			Status: types.EvaluationStatuePending, StartTime: time.Now(), Total: snapshot.Total,
		},
		Params:  e.evaluationParams(chatModelID, rerankModelID),
		Results: make([]types.EvaluationItemResult, len(snapshot.Items)),
	}
	for i, item := range snapshot.Items {
		detail.Results[i] = types.EvaluationItemResult{
			QID: item.QID, Question: item.Question, ReferenceAnswer: item.Answer, PIDs: item.PIDs, Passages: item.Passages,
			Status: "pending", Steps: make([]types.PipelineTraceStep, 0),
		}
	}
	if err := e.repository.Save(ctx, detail); err != nil {
		cleanupErr := e.knowledgeBaseService.DeleteKnowledgeBase(ctx, createdKB.ID)
		return nil, errors.Join(err, cleanupErr)
	}
	workerCtx := logger.CloneContext(ctx)
	go e.runEvaluation(workerCtx, detail, createdKB.ID, snapshot)
	return detail, nil
}

func resolveEvaluationModel(models []*types.Model, modelType types.ModelType, requested string) (string, error) {
	var selected string
	for _, model := range models {
		if model == nil || model.Type != modelType {
			continue
		}
		if requested != "" {
			if model.ID == requested {
				return requested, nil
			}
			continue
		}
		if model.IsDefault {
			return model.ID, nil
		}
		if selected == "" {
			selected = model.ID
		}
	}
	if requested != "" {
		return "", apperrors.NewBadRequestError(fmt.Sprintf("Evaluation model %s is unavailable or is not type %s", requested, modelType))
	}
	return selected, nil
}

func (e *EvaluationService) evaluationParams(chatModelID string, rerankModelID string) *types.ChatManage {
	return &types.ChatManage{
		PipelineRequest: types.PipelineRequest{
			VectorThreshold:  e.config.Conversation.VectorThreshold,
			KeywordThreshold: e.config.Conversation.KeywordThreshold,
			EmbeddingTopK:    e.config.Conversation.EmbeddingTopK,
			MaxRounds:        e.config.Conversation.MaxRounds,
			RerankModelID:    rerankModelID,
			RerankTopK:       e.config.Conversation.RerankTopK,
			RerankThreshold:  e.config.Conversation.RerankThreshold,
			ChatModelID:      chatModelID,
			SummaryConfig: types.SummaryConfig{
				MaxTokens:           e.config.Conversation.Summary.MaxTokens,
				RepeatPenalty:       e.config.Conversation.Summary.RepeatPenalty,
				TopK:                e.config.Conversation.Summary.TopK,
				TopP:                e.config.Conversation.Summary.TopP,
				Prompt:              e.config.Conversation.Summary.Prompt,
				ContextTemplate:     e.config.Conversation.Summary.ContextTemplate,
				FrequencyPenalty:    e.config.Conversation.Summary.FrequencyPenalty,
				PresencePenalty:     e.config.Conversation.Summary.PresencePenalty,
				NoMatchPrefix:       e.config.Conversation.Summary.NoMatchPrefix,
				Temperature:         e.config.Conversation.Summary.Temperature,
				Seed:                e.config.Conversation.Summary.Seed,
				MaxCompletionTokens: e.config.Conversation.Summary.MaxCompletionTokens,
			},
			FallbackResponse:    e.config.Conversation.FallbackResponse,
			RewritePromptSystem: e.config.Conversation.RewritePromptSystem,
			RewritePromptUser:   e.config.Conversation.RewritePromptUser,
		},
	}
}

func (e *EvaluationService) updateRun(ctx context.Context, taskID string, update func(*types.EvaluationDetail)) error {
	e.updateMu.Lock()
	defer e.updateMu.Unlock()
	detail, err := e.repository.Get(ctx, types.MustTenantIDFromContext(ctx), taskID)
	if err != nil {
		return err
	}
	if detail.Task.Status == types.EvaluationStatueSuccess || detail.Task.Status == types.EvaluationStatueFailed {
		return apperrors.NewConflictError("Evaluation task is already complete")
	}
	update(detail)
	return e.repository.Save(ctx, detail)
}

func (e *EvaluationService) runEvaluation(ctx context.Context, detail *types.EvaluationDetail, knowledgeBaseID string, snapshot *types.EvaluationDataset) {
	taskID := detail.Task.ID
	workerCtx, cancelWorker := context.WithCancel(ctx)
	defer cancelWorker()
	leaseCtx, stopLease := context.WithCancel(ctx)
	leaseResult := make(chan error, 1)
	go func() {
		err := e.maintainEvaluationLease(leaseCtx, taskID)
		if err != nil {
			cancelWorker()
		}
		leaseResult <- err
	}()
	runErr := e.executeEvaluation(workerCtx, detail, knowledgeBaseID, snapshot)
	if cleanupErr := e.knowledgeBaseService.DeleteKnowledgeBase(ctx, knowledgeBaseID); cleanupErr != nil {
		runErr = errors.Join(runErr, fmt.Errorf("delete evaluation knowledge base %s: %w", knowledgeBaseID, cleanupErr))
	}
	stopLease()
	runErr = errors.Join(runErr, <-leaseResult)
	err := e.updateRun(ctx, taskID, func(current *types.EvaluationDetail) {
		ended := time.Now()
		current.Task.EndTime = &ended
		current.Task.Status = types.EvaluationStatueSuccess
		if runErr != nil {
			current.Task.Status = types.EvaluationStatueFailed
			current.Task.ErrMsg = runErr.Error()
			for i := range current.Results {
				if current.Results[i].Status == "pending" || current.Results[i].Status == "running" {
					current.Results[i].Status = "failed"
					current.Results[i].Error = runErr.Error()
					current.Task.Failed++
					current.Task.Finished++
				}
			}
		} else if current.Task.Failed > 0 {
			current.Task.Status = types.EvaluationStatueFailed
			current.Task.ErrMsg = fmt.Sprintf("%d of %d evaluation questions failed; inspect question errors", current.Task.Failed, current.Task.Total)
		}
	})
	if err != nil {
		logger.ErrorWithFields(ctx, err, map[string]interface{}{"task_id": taskID, "operation": "save_evaluation_result"})
	}
	if runErr != nil {
		logger.ErrorWithFields(ctx, runErr, map[string]interface{}{"task_id": taskID, "operation": "execute_evaluation"})
	}
}

// maintainEvaluationLease 长索引阶段也续租，避免跨实例误判 / Renews leases even during long indexing stages.
func (e *EvaluationService) maintainEvaluationLease(ctx context.Context, taskID string) error {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if err := e.repository.Heartbeat(ctx, types.MustTenantIDFromContext(ctx), taskID); err != nil {
				if ctx.Err() != nil {
					return nil
				}
				return err
			}
		}
	}
}

func (e *EvaluationService) executeEvaluation(ctx context.Context, detail *types.EvaluationDetail, knowledgeBaseID string, snapshot *types.EvaluationDataset) (runErr error) {
	taskID := detail.Task.ID
	if err := e.updateRun(ctx, taskID, func(current *types.EvaluationDetail) { current.Task.Status = types.EvaluationStatueRunning }); err != nil {
		return err
	}
	passages := make([]string, len(snapshot.Corpus))
	for i, passage := range snapshot.Corpus {
		passages[i] = passage.Text
	}
	knowledge, err := e.knowledgeService.CreateKnowledgeFromPassageSync(ctx, knowledgeBaseID, passages, "")
	if err != nil {
		return fmt.Errorf("index evaluation corpus: %w", err)
	}
	if knowledge == nil {
		return fmt.Errorf("index evaluation corpus: knowledge service returned no knowledge record")
	}
	defer func() {
		if err := deleteReferencedKnowledge(ctx, e.knowledgeService, knowledgeBaseID, []string{knowledge.ID}); err != nil {
			runErr = errors.Join(runErr, fmt.Errorf("delete evaluation corpus %s: %w", knowledge.ID, err))
		}
	}()
	if err := validateEvaluationCorpusIndex(knowledge); err != nil {
		return err
	}
	var group errgroup.Group
	group.SetLimit(max(runtime.GOMAXPROCS(0)-1, 1))
	for index, item := range snapshot.Items {
		index, item := index, item
		group.Go(func() error {
			if err := e.updateRun(ctx, taskID, func(current *types.EvaluationDetail) { current.Results[index].Status = "running" }); err != nil {
				return err
			}
			result := e.evaluateQuestion(ctx, detail.Params, knowledgeBaseID, item, snapshot.Corpus)
			return e.updateRun(ctx, taskID, func(current *types.EvaluationDetail) {
				current.Results[index] = result
				current.Task.Finished++
				if result.Status == "failed" {
					current.Task.Failed++
				}
				current.Metric = averageEvaluationResults(current.Results)
			})
		})
	}
	return group.Wait()
}

// validateEvaluationCorpusIndex 校验已可检索的索引，允许摘要仍异步处理 / Validates retrievable indexes while summaries may still run asynchronously.
func validateEvaluationCorpusIndex(knowledge *types.Knowledge) error {
	switch knowledge.ParseStatus {
	case types.ParseStatusProcessing, types.ParseStatusFinalizing, types.ParseStatusCompleted:
		if knowledge.EnableStatus == "enabled" && knowledge.ProcessedAt != nil {
			return nil
		}
	}
	return fmt.Errorf("index evaluation corpus %s is not ready: status=%q, enable_status=%q, reason=%s", knowledge.ID, knowledge.ParseStatus, knowledge.EnableStatus, knowledge.ErrorMessage)
}

func (e *EvaluationService) evaluateQuestion(ctx context.Context, params *types.ChatManage, knowledgeBaseID string, item types.EvaluationDatasetItem, corpus []types.EvaluationPassage) types.EvaluationItemResult {
	started := time.Now()
	result := types.EvaluationItemResult{
		QID: item.QID, Question: item.Question, ReferenceAnswer: item.Answer, PIDs: item.PIDs, Passages: item.Passages,
		Status: "running", Steps: make([]types.PipelineTraceStep, 0),
	}
	chatManage := params.Clone()
	chatManage.Query = item.Question
	chatManage.RewriteQuery = item.Question
	chatManage.KnowledgeBaseIDs = []string{knowledgeBaseID}
	chatManage.SearchTargets = types.SearchTargets{&types.SearchTarget{Type: types.SearchTargetTypeKnowledgeBase, KnowledgeBaseID: knowledgeBaseID}}
	chatManage.PipelineTraceObserver = func(step types.PipelineTraceStep) {
		if types.EventType(step.ID) == types.CHUNK_RERANK && chatManage.RerankDiagnostics != nil {
			diagnostics := chatManage.RerankDiagnostics
			switch diagnostics.Outcome {
			case types.RerankOutcomeNoModel, types.RerankOutcomeDisabled, types.RerankOutcomeNoCandidates:
				result.Steps = append(result.Steps, step)
				return
			case types.RerankOutcomeModelError, types.RerankOutcomeModelUnavailable:
				step.Status = "failed"
				step.Error = fmt.Sprintf("Rerank model %s failed: %s", diagnostics.ModelID, diagnostics.Error)
				result.Error = step.Error
			}
		}
		if step.Status != "success" {
			result.Steps = append(result.Steps, step)
			return
		}
		switch types.EventType(step.ID) {
		case types.CHUNK_SEARCH, types.CHUNK_SEARCH_PARALLEL, types.CHUNK_RERANK:
			step.Metric = calculateEvaluationMetrics(item, step.Documents, "", corpus)
			step.Metric.GenerationMetrics = types.GenerationMetrics{}
			step.MetricScope = "retrieval"
		case types.CHAT_COMPLETION, types.EventType("fallback_response"):
			if chatManage.ChatResponse != nil {
				step.Metric = calculateEvaluationMetrics(item, nil, chatManage.ChatResponse.Content, corpus)
				step.Metric.RetrievalMetrics = types.RetrievalMetrics{}
				step.MetricScope = "generation"
			}
		}
		result.Steps = append(result.Steps, step)
	}
	err := e.sessionService.KnowledgeQAByEvent(ctx, chatManage, types.Pipeline["rag"])
	result.DurationMS = time.Since(started).Milliseconds()
	if chatManage.ChatResponse != nil {
		result.Answer = chatManage.ChatResponse.Content
		usage := chatManage.ChatResponse.Usage
		result.Usage = &usage
	}
	if err != nil {
		result.Status = "failed"
		result.Error = err.Error()
		return result
	}
	if result.Error != "" {
		result.Status = "failed"
		return result
	}
	if chatManage.ChatResponse == nil {
		result.Status = "failed"
		result.Error = "Question pipeline completed without an answer response"
		return result
	}
	result.Status = "success"
	documents := snapshotPipelineDocuments(chatManage.SearchResult)
	if chatManage.RerankResult != nil {
		documents = snapshotPipelineDocuments(chatManage.RerankResult)
	}
	result.Metric = calculateEvaluationMetrics(item, documents, result.Answer, corpus)
	return result
}
