package handler

import (
	"encoding/json"
	stderrors "errors"
	"io"
	"net/http"
	"unicode/utf8"

	"github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
)

type evaluationDatasetCreateRequest struct {
	Name        string                         `json:"name"`
	Description string                         `json:"description"`
	Content     types.EvaluationDatasetContent `json:"content"`
}

type evaluationDatasetSaveRequest struct {
	Revision    *int                           `json:"revision"`
	Name        string                         `json:"name"`
	Description string                         `json:"description"`
	Content     types.EvaluationDatasetContent `json:"content"`
}

type evaluationDatasetPublishRequest struct {
	Revision *int `json:"revision"`
}

// decodeEvaluationDatasetRequest 先限流量再校验UTF-8和强类型JSON / Bounds the body before validating UTF-8 and typed JSON.
func decodeEvaluationDatasetRequest[T evaluationDatasetCreateRequest | evaluationDatasetSaveRequest | evaluationDatasetPublishRequest](c *gin.Context, target *T) error {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, types.EvaluationDatasetMaxRequestBytes)
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		var oversized *http.MaxBytesError
		if stderrors.As(err, &oversized) {
			return &errors.AppError{Code: errors.ErrBadRequest, Message: "Dataset request exceeds 10 MiB", HTTPCode: http.StatusRequestEntityTooLarge}
		}
		return errors.NewBadRequestError("Cannot read dataset request").WithDetails(err.Error())
	}
	if !utf8.Valid(body) {
		return errors.NewBadRequestError("Dataset request must contain valid UTF-8 JSON")
	}
	if err := json.Unmarshal(body, target); err != nil {
		return errors.NewBadRequestError("Invalid dataset JSON").WithDetails(err.Error())
	}
	return nil
}

func (e *EvaluationHandler) CreateEvaluationDataset(c *gin.Context) {
	var request evaluationDatasetCreateRequest
	if err := decodeEvaluationDatasetRequest(c, &request); err != nil {
		c.Error(err)
		return
	}
	draft, err := e.datasetService.CreateEvaluationDataset(c.Request.Context(), request.Name, request.Description, request.Content)
	if err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"success": true, "data": draft})
}

func (e *EvaluationHandler) GetEvaluationDatasetDraft(c *gin.Context) {
	draft, err := e.datasetService.GetEvaluationDatasetDraft(c.Request.Context(), c.Param("id"))
	if err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": draft})
}

func (e *EvaluationHandler) SaveEvaluationDatasetDraft(c *gin.Context) {
	var request evaluationDatasetSaveRequest
	if err := decodeEvaluationDatasetRequest(c, &request); err != nil {
		c.Error(err)
		return
	}
	if request.Revision == nil || *request.Revision < 1 || *request.Revision > types.EvaluationDatasetMaxID {
		c.Error(errors.NewBadRequestError("A positive draft revision is required"))
		return
	}
	draft, err := e.datasetService.SaveEvaluationDatasetDraft(c.Request.Context(), c.Param("id"), *request.Revision, request.Name, request.Description, request.Content)
	if err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": draft})
}

func (e *EvaluationHandler) PublishEvaluationDataset(c *gin.Context) {
	var request evaluationDatasetPublishRequest
	if err := decodeEvaluationDatasetRequest(c, &request); err != nil {
		c.Error(err)
		return
	}
	if request.Revision == nil || *request.Revision < 1 || *request.Revision > types.EvaluationDatasetMaxID {
		c.Error(errors.NewBadRequestError("A positive draft revision is required"))
		return
	}
	version, err := e.datasetService.PublishEvaluationDataset(c.Request.Context(), c.Param("id"), *request.Revision)
	if err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"success": true, "data": version})
}

func (e *EvaluationHandler) ListEvaluationDatasetVersions(c *gin.Context) {
	versions, err := e.datasetService.ListEvaluationDatasetVersions(c.Request.Context(), c.Param("id"))
	if err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": versions})
}
