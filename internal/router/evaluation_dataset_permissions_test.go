package router

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Tencent/WeKnora/internal/handler"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestEvaluationDatasetWritesRequireFullAccessAPIKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	guards := &rbacGuards{}
	RegisterEvaluationRoutes(gin.New().Group("/api/v1"), &handler.EvaluationHandler{}, guards)
	cases := []struct {
		method     string
		path       string
		fullAccess bool
		expected   int
	}{
		{http.MethodPost, "/api/v1/evaluation/datasets", false, http.StatusForbidden},
		{http.MethodPut, "/api/v1/evaluation/datasets/:id/draft", false, http.StatusForbidden},
		{http.MethodPost, "/api/v1/evaluation/datasets/:id/publish", false, http.StatusForbidden},
		{http.MethodPost, "/api/v1/evaluation/datasets", true, http.StatusNoContent},
		{http.MethodGet, "/api/v1/evaluation/datasets/:id/draft", false, http.StatusNoContent},
		{http.MethodGet, "/api/v1/evaluation/datasets/:id/versions", false, http.StatusNoContent},
	}
	for _, test := range cases {
		t.Run(test.method+test.path, func(t *testing.T) {
			server := gin.New()
			server.Use(func(c *gin.Context) {
				scope := types.TenantAPIKeyScope{FullAccess: test.fullAccess, Capabilities: types.StringArray{string(types.APIKeyCapabilityRunEvaluations)}}
				c.Request = c.Request.WithContext(types.WithTenantAPIKeyScope(c.Request.Context(), scope))
				c.Next()
			})
			server.Use(guards.apiKeyAuthorizer.Middleware())
			server.Handle(test.method, test.path, func(c *gin.Context) { c.Status(http.StatusNoContent) })
			response := httptest.NewRecorder()
			server.ServeHTTP(response, httptest.NewRequest(test.method, strings.ReplaceAll(test.path, ":id", "dataset-1"), nil))
			require.Equal(t, test.expected, response.Code, response.Body.String())
		})
	}
}
