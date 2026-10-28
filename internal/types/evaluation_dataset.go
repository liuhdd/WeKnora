package types

import (
	"encoding/json"
	"fmt"
	"time"
)

const (
	EvaluationDatasetMaxQuestions             = 1000
	EvaluationDatasetMaxPassages              = 5000
	EvaluationDatasetMaxTextBytes             = 65536
	EvaluationDatasetMaxContentBytes          = 8 << 20
	EvaluationDatasetMaxExpandedEvidenceBytes = 8 << 20
	EvaluationDatasetMaxRequestBytes          = 10 << 20
	EvaluationDatasetMaxID                    = 9007199254740991
)

// EvaluationDatasetContent 保存完整问题和语料，包含干扰段落 / Preserves every question and corpus passage, including distractors.
type EvaluationDatasetContent struct {
	Questions []EvaluationDatasetQuestion `json:"questions"`
	Corpus    []EvaluationPassage         `json:"corpus"`
}

type EvaluationDatasetQuestion struct {
	QID      int    `json:"qid"`
	Question string `json:"question"`
	Answer   string `json:"answer"`
	PIDs     []int  `json:"pids"`
}

// UnmarshalJSON 区分缺失字段与合法的零编号和空草稿文本 / Distinguishes missing fields from valid zero IDs and empty draft text.
func (q *EvaluationDatasetQuestion) UnmarshalJSON(data []byte) error {
	var value struct {
		QID      *int    `json:"qid"`
		Question *string `json:"question"`
		Answer   *string `json:"answer"`
		PIDs     *[]*int `json:"pids"`
	}
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	if value.QID == nil || value.Question == nil || value.Answer == nil || value.PIDs == nil {
		return fmt.Errorf("each dataset question requires qid, question, answer and pids")
	}
	pids := make([]int, len(*value.PIDs))
	for index, pid := range *value.PIDs {
		if pid == nil {
			return fmt.Errorf("question %d pids[%d] must be an integer, not null", *value.QID, index)
		}
		pids[index] = *pid
	}
	*q = EvaluationDatasetQuestion{QID: *value.QID, Question: *value.Question, Answer: *value.Answer, PIDs: pids}
	return nil
}

// UnmarshalJSON 保证语料字段存在，不隐式修复数据 / Requires corpus fields without silently repairing input.
func (p *EvaluationPassage) UnmarshalJSON(data []byte) error {
	var value struct {
		ID   *int    `json:"id"`
		Text *string `json:"text"`
	}
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	if value.ID == nil || value.Text == nil {
		return fmt.Errorf("each corpus passage requires id and text")
	}
	*p = EvaluationPassage{ID: *value.ID, Text: *value.Text}
	return nil
}

type EvaluationDatasetDraft struct {
	ID          string                   `json:"id"`
	Name        string                   `json:"name"`
	Description string                   `json:"description"`
	Revision    int                      `json:"revision"`
	Content     EvaluationDatasetContent `json:"content"`
	UpdatedAt   time.Time                `json:"updated_at"`
}

type EvaluationDatasetVersion struct {
	Version     string     `json:"version"`
	ContentHash string     `json:"content_hash"`
	Total       int        `json:"total"`
	CreatedAt   *time.Time `json:"created_at,omitempty"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
}

type EvaluationPublishedDataset struct {
	ID string
	EvaluationDatasetVersion
	Content EvaluationDatasetContent
}

// EvaluationDatasetRecord 只有草稿可被修改 / Only draft state is mutable.
type EvaluationDatasetRecord struct {
	ID             string `gorm:"type:varchar(36);primaryKey"`
	TenantID       uint64 `gorm:"index:idx_evaluation_datasets_tenant_updated,priority:1"`
	Name           string `gorm:"type:varchar(800)"`
	Description    string `gorm:"type:text"`
	DraftRevision  int
	DraftContent   string `gorm:"type:text"`
	DraftTotal     int
	LatestVersion  int
	PublishedHash  string `gorm:"type:varchar(64)"`
	PublishedTotal int
	CreatedAt      time.Time
	UpdatedAt      time.Time `gorm:"index:idx_evaluation_datasets_tenant_updated,priority:2,sort:desc"`
}

func (EvaluationDatasetRecord) TableName() string { return "evaluation_datasets" }

// EvaluationDatasetVersionRecord 发布后只插入不更新 / Published versions are inserted once and never updated.
type EvaluationDatasetVersionRecord struct {
	DatasetID   string `gorm:"type:varchar(36);primaryKey"`
	Version     int    `gorm:"primaryKey"`
	TenantID    uint64 `gorm:"index:idx_evaluation_dataset_versions_tenant"`
	Name        string `gorm:"type:varchar(800)"`
	Description string `gorm:"type:text"`
	Content     string `gorm:"type:text"`
	ContentHash string `gorm:"type:varchar(64)"`
	Total       int
	CreatedAt   time.Time
}

func (EvaluationDatasetVersionRecord) TableName() string { return "evaluation_dataset_versions" }
