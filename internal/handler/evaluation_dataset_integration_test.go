package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/middleware"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func evaluationDatasetHTTPServer(t *testing.T) (*gin.Engine, *gorm.DB) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "http-datasets.sqlite")), &gorm.Config{})
	require.NoError(t, err)
	migration, err := os.ReadFile(filepath.Join("..", "..", "migrations", "sqlite", "000052_evaluation_datasets.up.sql"))
	require.NoError(t, err)
	require.NoError(t, db.Exec(string(migration)).Error)
	connection, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, connection.Close()) })
	dataset := service.NewDatasetService(repository.NewEvaluationDatasetRepository(db))
	evaluation := service.NewEvaluationService(nil, dataset, nil, nil, nil, nil, repository.NewEvaluationRepository(db))
	handler := NewEvaluationHandler(evaluation, dataset)
	server := gin.New()
	server.Use(middleware.ErrorHandler())
	server.Use(func(c *gin.Context) {
		c.Set(string(types.TenantIDContextKey), uint64(1))
		c.Request = c.Request.WithContext(types.WithExecutionTenant(c.Request.Context(), 1))
		c.Next()
	})
	server.POST("/evaluation/datasets", handler.CreateEvaluationDataset)
	server.GET("/evaluation/datasets/:id/draft", handler.GetEvaluationDatasetDraft)
	server.PUT("/evaluation/datasets/:id/draft", handler.SaveEvaluationDatasetDraft)
	server.POST("/evaluation/datasets/:id/publish", handler.PublishEvaluationDataset)
	server.GET("/evaluation/datasets/:id/versions", handler.ListEvaluationDatasetVersions)
	server.GET("/evaluation/datasets/:id", handler.GetEvaluationDataset)
	server.POST("/evaluation", handler.Evaluation)
	return server, db
}

func datasetHTTPRequest(server *gin.Engine, method string, path string, body []byte) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, path, bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	return response
}

func TestEvaluationDatasetHTTPRejectsMissingFieldsNullsAndOversize(t *testing.T) {
	server, db := evaluationDatasetHTTPServer(t)
	valid := []byte(`{"name":"Draft","extra":{"ignored":true},"content":{"questions":[{"qid":0,"question":"","answer":"","pids":[]}],"corpus":[{"id":0,"text":""}]}}`)
	response := datasetHTTPRequest(server, http.MethodPost, "/evaluation/datasets", valid)
	require.Equal(t, http.StatusCreated, response.Code, response.Body.String())
	var created struct {
		Data types.EvaluationDatasetDraft `json:"data"`
	}
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &created))
	require.Equal(t, 0, created.Data.Content.Questions[0].QID)
	invalid := []string{
		`{"name":"Bad","content":{"questions":null,"corpus":[]}}`,
		`{"name":"Bad","content":{"questions":[{"question":"","answer":"","pids":[]}],"corpus":[]}}`,
		`{"name":"Bad","content":{"questions":[{"qid":0,"answer":"","pids":[]}],"corpus":[]}}`,
		`{"name":"Bad","content":{"questions":[{"qid":0,"question":"","answer":null,"pids":[]}],"corpus":[]}}`,
		`{"name":"Bad","content":{"questions":[{"qid":0,"question":"","answer":"","pids":[null]}],"corpus":[{"id":0,"text":""}]}}`,
		`{"name":"Bad","content":{"questions":[],"corpus":[{"text":""}]}}`,
		`{"name":"Bad","content":{"questions":[],"corpus":[{"id":0,"text":null}]}}`,
		`{"name":"Bad","content":{"questions":[{"qid":9007199254740992,"question":"","answer":"","pids":[]}],"corpus":[]}}`,
	}
	for _, body := range invalid {
		response = datasetHTTPRequest(server, http.MethodPost, "/evaluation/datasets", []byte(body))
		require.Equal(t, http.StatusBadRequest, response.Code, response.Body.String())
	}
	response = datasetHTTPRequest(server, http.MethodPost, "/evaluation/datasets", []byte(strings.Repeat(" ", types.EvaluationDatasetMaxRequestBytes+1)))
	require.Equal(t, http.StatusRequestEntityTooLarge, response.Code, response.Body.String())
	var count int64
	require.NoError(t, db.Model(&types.EvaluationDatasetRecord{}).Count(&count).Error)
	require.EqualValues(t, 1, count)
	response = datasetHTTPRequest(server, http.MethodPost, "/evaluation/datasets/"+created.Data.ID+"/publish", []byte(`{"revision":1}`))
	require.Equal(t, http.StatusBadRequest, response.Code, response.Body.String())
	response = datasetHTTPRequest(server, http.MethodPost, "/evaluation", []byte(`{"dataset_id":"`+created.Data.ID+`"}`))
	require.Equal(t, http.StatusNotFound, response.Code, response.Body.String())
}

func TestEvaluationDatasetHTTPPublishFreezesVersionAndReportsConflicts(t *testing.T) {
	server, _ := evaluationDatasetHTTPServer(t)
	body := []byte(`{"name":"Backups","description":"Versioned","content":{"questions":[{"qid":0,"question":"Retention?","answer":"Seven days","pids":[0]}],"corpus":[{"id":0,"text":"Seven days"},{"id":1,"text":"Unrelated guidance"}]}}`)
	response := datasetHTTPRequest(server, http.MethodPost, "/evaluation/datasets", body)
	require.Equal(t, http.StatusCreated, response.Code, response.Body.String())
	var created struct {
		Data types.EvaluationDatasetDraft `json:"data"`
	}
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &created))
	path := "/evaluation/datasets/" + created.Data.ID
	response = datasetHTTPRequest(server, http.MethodPost, path+"/publish", []byte(`{"revision":1}`))
	require.Equal(t, http.StatusCreated, response.Code, response.Body.String())
	response = datasetHTTPRequest(server, http.MethodPost, path+"/publish", []byte(`{"revision":1}`))
	require.Equal(t, http.StatusConflict, response.Code, response.Body.String())
	response = datasetHTTPRequest(server, http.MethodGet, path+"?version=v1", nil)
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	var preview struct {
		Data types.EvaluationDataset `json:"data"`
	}
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &preview))
	require.Equal(t, "v1", preview.Data.PublishedVersion)
	require.Len(t, preview.Data.Version, 64)
	require.Len(t, preview.Data.Corpus, 2)
	require.Equal(t, "Unrelated guidance", preview.Data.Corpus[1].Text)
	response = datasetHTTPRequest(server, http.MethodPut, "/evaluation/datasets/default/draft", body)
	require.Equal(t, http.StatusBadRequest, response.Code, response.Body.String())
}
