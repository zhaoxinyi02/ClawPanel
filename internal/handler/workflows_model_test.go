package handler

import (
	"os"
	"testing"

	"github.com/zhaoxinyi02/ClawPanel/internal/model"
)

func TestResolveWorkflowModelSelectionFromDefaultsString(t *testing.T) {
	t.Parallel()

	settings := &model.WorkflowSettings{}
	ocConfig := map[string]interface{}{
		"agents": map[string]interface{}{
			"defaults": map[string]interface{}{
				"model": "openai/gpt-5.4",
			},
		},
		"models": map[string]interface{}{
			"providers": map[string]interface{}{
				"openai": map[string]interface{}{
					"models": []interface{}{"gpt-5.4"},
				},
			},
		},
	}

	pid, mid := resolveWorkflowModelSelection(settings, ocConfig)
	if pid != "openai" || mid != "gpt-5.4" {
		t.Fatalf("expected openai/gpt-5.4, got %q/%q", pid, mid)
	}
}

func TestResolveWorkflowModelSelectionFillsMissingModelID(t *testing.T) {
	t.Parallel()

	settings := &model.WorkflowSettings{ProviderID: "openai", ModelID: ""}
	ocConfig := map[string]interface{}{
		"models": map[string]interface{}{
			"providers": map[string]interface{}{
				"openai": map[string]interface{}{
					"models": []interface{}{
						map[string]interface{}{"id": "gpt-5.4"},
					},
				},
			},
		},
	}

	pid, mid := resolveWorkflowModelSelection(settings, ocConfig)
	if pid != "openai" || mid != "gpt-5.4" {
		t.Fatalf("expected openai/gpt-5.4, got %q/%q", pid, mid)
	}
}

func TestResolveWorkflowAPIKeyFromEnvObject(t *testing.T) {
	t.Parallel()

	const envKey = "CLAWPANEL_WORKFLOW_TEST_KEY"
	if err := os.Setenv(envKey, "env-secret"); err != nil {
		t.Fatalf("setenv: %v", err)
	}
	t.Cleanup(func() { _ = os.Unsetenv(envKey) })

	got := resolveWorkflowAPIKey(map[string]interface{}{"env": envKey})
	if got != "env-secret" {
		t.Fatalf("expected env-secret, got %q", got)
	}
}

func TestResolveWorkflowAPIKeyFromValueObject(t *testing.T) {
	t.Parallel()

	got := resolveWorkflowAPIKey(map[string]interface{}{"value": "inline-secret"})
	if got != "inline-secret" {
		t.Fatalf("expected inline-secret, got %q", got)
	}
}
