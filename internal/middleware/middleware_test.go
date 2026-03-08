package middleware

import (
	"bytes"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/zhaoxinyi02/ClawPanel/internal/config"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func testConfig() *config.Config {
	return &config.Config{JWTSecret: "test-secret"}
}

func TestGenerateTokenAndValidateToken(t *testing.T) {
	token, err := GenerateToken("test-secret")
	if err != nil {
		t.Fatalf("GenerateToken() error = %v", err)
	}
	if token == "" {
		t.Fatal("GenerateToken() returned empty token")
	}
	if !ValidateToken(token, "test-secret") {
		t.Fatal("ValidateToken() should accept generated token")
	}
	if ValidateToken(token, "wrong-secret") {
		t.Fatal("ValidateToken() should reject token signed with another secret")
	}
	if ValidateToken("", "test-secret") {
		t.Fatal("ValidateToken() should reject empty token")
	}
}

func TestAuthRequiresToken(t *testing.T) {
	router := gin.New()
	called := false
	router.GET("/protected", Auth(testConfig()), func(c *gin.Context) {
		called = true
		c.Status(http.StatusNoContent)
	})

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusUnauthorized)
	}
	if called {
		t.Fatal("protected handler should not run without token")
	}
	if !strings.Contains(recorder.Body.String(), "未提供认证令牌") {
		t.Fatalf("unexpected body: %s", recorder.Body.String())
	}
}

func TestAuthAcceptsBearerTokenAndSetsRole(t *testing.T) {
	token, err := GenerateToken(testConfig().JWTSecret)
	if err != nil {
		t.Fatalf("GenerateToken() error = %v", err)
	}

	router := gin.New()
	router.GET("/protected", Auth(testConfig()), func(c *gin.Context) {
		role, ok := c.Get("role")
		if !ok {
			t.Fatal("expected role in context")
		}
		c.String(http.StatusOK, role.(string))
	})

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if strings.TrimSpace(recorder.Body.String()) != "admin" {
		t.Fatalf("role = %q, want admin", recorder.Body.String())
	}
}

func TestAuthAcceptsQueryToken(t *testing.T) {
	token, err := GenerateToken(testConfig().JWTSecret)
	if err != nil {
		t.Fatalf("GenerateToken() error = %v", err)
	}

	router := gin.New()
	router.GET("/protected", Auth(testConfig()), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/protected?token="+token, nil)
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNoContent)
	}
}

func TestCORSHandlesOptionsAndSetsHeaders(t *testing.T) {
	router := gin.New()
	router.Use(CORS())
	router.Any("/ping", func(c *gin.Context) {
		c.String(http.StatusOK, "pong")
	})

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodOptions, "/ping", nil)
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNoContent)
	}
	if got := recorder.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Fatalf("allow origin = %q, want *", got)
	}
	if got := recorder.Header().Get("Access-Control-Allow-Methods"); !strings.Contains(got, "OPTIONS") {
		t.Fatalf("allow methods = %q, expected OPTIONS", got)
	}
}

func TestLoggerSkipsWebSocketLogPath(t *testing.T) {
	var buf bytes.Buffer
	origWriter := log.Writer()
	origFlags := log.Flags()
	log.SetOutput(&buf)
	log.SetFlags(0)
	defer func() {
		log.SetOutput(origWriter)
		log.SetFlags(origFlags)
	}()

	router := gin.New()
	router.Use(Logger())
	router.GET("/api/ws/logs", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/ws/logs", nil)
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if buf.Len() != 0 {
		t.Fatalf("expected websocket log path to be skipped, got %q", buf.String())
	}
}

func TestLoggerLogsErrorResponses(t *testing.T) {
	var buf bytes.Buffer
	origWriter := log.Writer()
	origFlags := log.Flags()
	log.SetOutput(&buf)
	log.SetFlags(0)
	defer func() {
		log.SetOutput(origWriter)
		log.SetFlags(origFlags)
	}()

	router := gin.New()
	router.Use(Logger())
	router.GET("/fail", func(c *gin.Context) {
		c.String(http.StatusInternalServerError, "boom")
	})

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/fail", nil)
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusInternalServerError)
	}
	logLine := buf.String()
	if !strings.Contains(logLine, "GET /fail") || !strings.Contains(logLine, "500") {
		t.Fatalf("unexpected log line: %q", logLine)
	}
}
