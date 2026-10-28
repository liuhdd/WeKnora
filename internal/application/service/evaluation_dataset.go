package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"

	apperrors "github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/types"
)

// ValidateEvaluationDatasetDraft 校验结构和上限，保留可继续编辑的空文本 / Validates structure and limits while allowing incomplete draft text.
func ValidateEvaluationDatasetDraft(name string, description string, content types.EvaluationDatasetContent) error {
	if strings.TrimSpace(name) == "" || utf8.RuneCountInString(name) > 200 {
		return apperrors.NewBadRequestError("Dataset name must contain 1–200 characters")
	}
	if utf8.RuneCountInString(description) > 2000 {
		return apperrors.NewBadRequestError("Dataset description exceeds 2000 characters")
	}
	if content.Questions == nil || content.Corpus == nil {
		return apperrors.NewBadRequestError("Dataset content requires non-null questions and corpus arrays")
	}
	if len(content.Questions) > types.EvaluationDatasetMaxQuestions || len(content.Corpus) > types.EvaluationDatasetMaxPassages {
		return apperrors.NewBadRequestError("Dataset exceeds 1000 questions or 5000 corpus passages")
	}
	passages := make(map[int]struct{}, len(content.Corpus))
	for index, passage := range content.Corpus {
		if passage.ID < 0 || passage.ID > types.EvaluationDatasetMaxID {
			return apperrors.NewBadRequestError(fmt.Sprintf("Corpus row %d has an invalid ID; use a nonnegative JavaScript safe integer", index+1))
		}
		if _, exists := passages[passage.ID]; exists {
			return apperrors.NewBadRequestError(fmt.Sprintf("Duplicate corpus passage ID %d", passage.ID))
		}
		if len(passage.Text) > types.EvaluationDatasetMaxTextBytes {
			return apperrors.NewBadRequestError(fmt.Sprintf("Corpus passage %d exceeds 65536 UTF-8 bytes", passage.ID))
		}
		passages[passage.ID] = struct{}{}
	}
	questions := make(map[int]struct{}, len(content.Questions))
	for index, question := range content.Questions {
		if question.QID < 0 || question.QID > types.EvaluationDatasetMaxID {
			return apperrors.NewBadRequestError(fmt.Sprintf("Question row %d has an invalid qid; use a nonnegative JavaScript safe integer", index+1))
		}
		if _, exists := questions[question.QID]; exists {
			return apperrors.NewBadRequestError(fmt.Sprintf("Duplicate question ID %d", question.QID))
		}
		questions[question.QID] = struct{}{}
		if question.PIDs == nil {
			return apperrors.NewBadRequestError(fmt.Sprintf("Question %d requires a non-null pids array", question.QID))
		}
		if len(question.Question) > types.EvaluationDatasetMaxTextBytes || len(question.Answer) > types.EvaluationDatasetMaxTextBytes {
			return apperrors.NewBadRequestError(fmt.Sprintf("Question %d question or answer exceeds 65536 UTF-8 bytes", question.QID))
		}
		references := make(map[int]struct{}, len(question.PIDs))
		for _, pid := range question.PIDs {
			if _, exists := passages[pid]; !exists {
				return apperrors.NewBadRequestError(fmt.Sprintf("Question %d references missing corpus passage %d", question.QID, pid))
			}
			if _, exists := references[pid]; exists {
				return apperrors.NewBadRequestError(fmt.Sprintf("Question %d repeats passage reference %d", question.QID, pid))
			}
			references[pid] = struct{}{}
		}
	}
	payload, err := json.Marshal(content)
	if err != nil {
		return fmt.Errorf("encode dataset content: %w", err)
	}
	if len(payload) > types.EvaluationDatasetMaxContentBytes {
		return apperrors.NewBadRequestError("Dataset content exceeds 8 MiB")
	}
	return nil
}

// ValidatePublishedEvaluationDataset 只在发布时要求完整可评测内容 / Requires complete evaluable content only at publication.
func ValidatePublishedEvaluationDataset(content types.EvaluationDatasetContent) error {
	if len(content.Questions) == 0 {
		return apperrors.NewBadRequestError("Publish requires at least one question")
	}
	texts := make(map[string]int, len(content.Corpus))
	passageBytes := make(map[int]int, len(content.Corpus))
	for _, passage := range content.Corpus {
		if strings.TrimSpace(passage.Text) == "" {
			return apperrors.NewBadRequestError(fmt.Sprintf("Corpus passage %d is empty; add evidence text before publishing", passage.ID))
		}
		if previous, exists := texts[passage.Text]; exists {
			return apperrors.NewBadRequestError(fmt.Sprintf("Corpus passage %d duplicates passage %d text; reuse the existing passage ID", passage.ID, previous))
		}
		texts[passage.Text] = passage.ID
		passageBytes[passage.ID] = len(passage.Text)
	}
	expandedBytes := 0
	for _, question := range content.Questions {
		if strings.TrimSpace(question.Question) == "" || strings.TrimSpace(question.Answer) == "" || len(question.PIDs) == 0 {
			return apperrors.NewBadRequestError(fmt.Sprintf("Question %d requires a question, reference answer and evidence passages before publishing", question.QID))
		}
		for _, pid := range question.PIDs {
			expandedBytes += passageBytes[pid]
			if expandedBytes > types.EvaluationDatasetMaxExpandedEvidenceBytes {
				return apperrors.NewBadRequestError(fmt.Sprintf(
					"Question %d expands total referenced evidence beyond 8 MiB; reduce references or split the dataset",
					question.QID,
				))
			}
		}
	}
	return nil
}

func authorizeEvaluationDatasetDraft(ctx context.Context, datasetID string) error {
	if err := authorizeEvaluationWorkspace(ctx); err != nil {
		return err
	}
	if datasetID == "default" {
		return apperrors.NewForbiddenError("The built-in Parquet dataset is read-only; copy it to a new dataset to edit")
	}
	return nil
}

func (d *DatasetService) CreateEvaluationDataset(ctx context.Context, name string, description string, content types.EvaluationDatasetContent) (*types.EvaluationDatasetDraft, error) {
	if err := authorizeEvaluationWorkspace(ctx); err != nil {
		return nil, err
	}
	if err := ValidateEvaluationDatasetDraft(name, description, content); err != nil {
		return nil, err
	}
	return d.repository.Create(ctx, types.MustTenantIDFromContext(ctx), name, description, content)
}

func (d *DatasetService) GetEvaluationDatasetDraft(ctx context.Context, datasetID string) (*types.EvaluationDatasetDraft, error) {
	if err := authorizeEvaluationDatasetDraft(ctx, datasetID); err != nil {
		return nil, err
	}
	return d.repository.GetDraft(ctx, types.MustTenantIDFromContext(ctx), datasetID)
}

func (d *DatasetService) SaveEvaluationDatasetDraft(ctx context.Context, datasetID string, revision int, name string, description string, content types.EvaluationDatasetContent) (*types.EvaluationDatasetDraft, error) {
	if err := authorizeEvaluationDatasetDraft(ctx, datasetID); err != nil {
		return nil, err
	}
	if revision < 1 || revision > types.EvaluationDatasetMaxID {
		return nil, apperrors.NewBadRequestError("A positive draft revision is required")
	}
	if err := ValidateEvaluationDatasetDraft(name, description, content); err != nil {
		return nil, err
	}
	return d.repository.SaveDraft(ctx, types.MustTenantIDFromContext(ctx), datasetID, revision, name, description, content)
}

func (d *DatasetService) PublishEvaluationDataset(ctx context.Context, datasetID string, revision int) (*types.EvaluationDatasetVersion, error) {
	draft, err := d.GetEvaluationDatasetDraft(ctx, datasetID)
	if err != nil {
		return nil, err
	}
	if draft.Revision != revision {
		return nil, apperrors.NewConflictError("Dataset draft changed; reload it before publishing")
	}
	if err := ValidateEvaluationDatasetDraft(draft.Name, draft.Description, draft.Content); err != nil {
		return nil, err
	}
	if err := ValidatePublishedEvaluationDataset(draft.Content); err != nil {
		return nil, err
	}
	return d.repository.Publish(ctx, types.MustTenantIDFromContext(ctx), datasetID, revision)
}

func (d *DatasetService) ListEvaluationDatasets(ctx context.Context) ([]types.EvaluationDatasetSummary, error) {
	if err := authorizeEvaluationWorkspace(ctx); err != nil {
		return nil, err
	}
	builtin, err := d.GetEvaluationDataset(ctx, "default")
	if err != nil {
		return nil, err
	}
	managed, err := d.repository.List(ctx, types.MustTenantIDFromContext(ctx))
	if err != nil {
		return nil, err
	}
	return append([]types.EvaluationDatasetSummary{builtin.EvaluationDatasetSummary}, managed...), nil
}

func (d *DatasetService) GetEvaluationDatasetVersion(ctx context.Context, datasetID string, version string) (*types.EvaluationDataset, error) {
	if err := authorizeEvaluationWorkspace(ctx); err != nil {
		return nil, err
	}
	if datasetID == "default" {
		data, err := DefaultDataset()
		if err != nil {
			return nil, err
		}
		snapshot, err := evaluationDatasetSnapshot(data)
		if err != nil {
			return nil, err
		}
		if version != "" && version != snapshot.Version {
			return nil, apperrors.NewNotFoundError("Requested built-in dataset version is unavailable; its Parquet content has changed")
		}
		return snapshot, nil
	}
	number := 0
	if version != "" {
		parsed, err := strconv.Atoi(strings.TrimPrefix(version, "v"))
		if err != nil || parsed < 1 || version != fmt.Sprintf("v%d", parsed) {
			return nil, apperrors.NewBadRequestError("Published dataset version must use the label v1, v2, …")
		}
		number = parsed
	}
	published, err := d.repository.GetPublished(ctx, types.MustTenantIDFromContext(ctx), datasetID, number)
	if err != nil {
		return nil, err
	}
	return publishedEvaluationSnapshot(*published), nil
}

func publishedEvaluationSnapshot(published types.EvaluationPublishedDataset) *types.EvaluationDataset {
	passages := make(map[int]string, len(published.Content.Corpus))
	for _, passage := range published.Content.Corpus {
		passages[passage.ID] = passage.Text
	}
	items := make([]types.EvaluationDatasetItem, 0, len(published.Content.Questions))
	for _, question := range published.Content.Questions {
		evidence := make([]string, 0, len(question.PIDs))
		for _, pid := range question.PIDs {
			evidence = append(evidence, passages[pid])
		}
		items = append(items, types.EvaluationDatasetItem{QID: question.QID, Question: question.Question, Answer: question.Answer, PIDs: question.PIDs, Passages: evidence})
	}
	return &types.EvaluationDataset{
		EvaluationDatasetSummary: types.EvaluationDatasetSummary{
			ID: published.ID, Name: published.Name, Description: published.Description, Version: published.ContentHash,
			PublishedVersion: published.Version, ReadOnly: false, Total: published.Total,
		},
		Items: items, Corpus: published.Content.Corpus,
	}
}

func (d *DatasetService) ListEvaluationDatasetVersions(ctx context.Context, datasetID string) ([]types.EvaluationDatasetVersion, error) {
	if err := authorizeEvaluationWorkspace(ctx); err != nil {
		return nil, err
	}
	if datasetID == "default" {
		snapshot, err := d.GetEvaluationDataset(ctx, datasetID)
		if err != nil {
			return nil, err
		}
		return []types.EvaluationDatasetVersion{{Version: snapshot.PublishedVersion, ContentHash: snapshot.Version, Name: snapshot.Name, Description: snapshot.Description, Total: snapshot.Total}}, nil
	}
	return d.repository.ListVersions(ctx, types.MustTenantIDFromContext(ctx), datasetID)
}
