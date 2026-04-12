package handler

import (
	"testing"
)

func TestPreserveHiddenOpenClawFields_AllowAlsoAllowConflict(t *testing.T) {
	// Scenario: old config has alsoAllow, new config introduces allow via UI save.
	// alsoAllow items should be merged into allow, then alsoAllow removed.
	src := map[string]interface{}{
		"tools": map[string]interface{}{
			"profile":   "full",
			"alsoAllow": []interface{}{"feishu_bitable_app", "feishu_chat"},
			"exec":      map[string]interface{}{"ask": "off"},
		},
	}
	dst := map[string]interface{}{
		"tools": map[string]interface{}{
			"allow": []interface{}{"group:web", "group:fs"},
		},
	}

	preserveHiddenOpenClawFields(dst, src)

	dstTools := dst["tools"].(map[string]interface{})

	// alsoAllow must NOT be present (conflict resolved)
	if _, ok := dstTools["alsoAllow"]; ok {
		t.Fatal("'alsoAllow' should have been removed to avoid conflict with 'allow'")
	}

	// allow must contain original items + merged alsoAllow items
	allowList, ok := dstTools["allow"].([]interface{})
	if !ok {
		t.Fatal("expected 'allow' to be a list")
	}
	allowSet := make(map[string]bool)
	for _, v := range allowList {
		if s, ok := v.(string); ok {
			allowSet[s] = true
		}
	}
	for _, expected := range []string{"group:web", "group:fs", "feishu_bitable_app", "feishu_chat"} {
		if !allowSet[expected] {
			t.Errorf("expected '%s' in merged allow list", expected)
		}
	}

	// Other fields (profile, exec) should still be preserved from src
	if _, ok := dstTools["profile"]; !ok {
		t.Error("expected 'profile' to be preserved from src")
	}
	if _, ok := dstTools["exec"]; !ok {
		t.Error("expected 'exec' to be preserved from src")
	}
}

func TestPreserveHiddenOpenClawFields_AllowAlsoAllowDedup(t *testing.T) {
	// When alsoAllow has items already in allow, they should not be duplicated.
	src := map[string]interface{}{
		"tools": map[string]interface{}{
			"alsoAllow": []interface{}{"group:web", "feishu_chat"},
		},
	}
	dst := map[string]interface{}{
		"tools": map[string]interface{}{
			"allow": []interface{}{"group:web", "group:fs"},
		},
	}

	preserveHiddenOpenClawFields(dst, src)

	dstTools := dst["tools"].(map[string]interface{})
	allowList := dstTools["allow"].([]interface{})

	// Should have 3 unique items, not 4 (group:web deduplicated)
	if len(allowList) != 3 {
		t.Errorf("expected 3 items after dedup, got %d: %v", len(allowList), allowList)
	}
}

func TestPreserveHiddenOpenClawFields_OnlyAlsoAllow(t *testing.T) {
	// When only alsoAllow exists (no allow), it should be preserved normally.
	src := map[string]interface{}{
		"tools": map[string]interface{}{
			"profile":   "full",
			"alsoAllow": []interface{}{"feishu_bitable_app"},
		},
	}
	dst := map[string]interface{}{
		"tools": map[string]interface{}{
			"profile": "full",
		},
	}

	preserveHiddenOpenClawFields(dst, src)

	dstTools := dst["tools"].(map[string]interface{})
	if _, ok := dstTools["alsoAllow"]; !ok {
		t.Fatal("expected 'alsoAllow' to be preserved when 'allow' is absent")
	}
}

func TestPreserveHiddenOpenClawFields_OnlyAllow(t *testing.T) {
	// When only allow exists in both, no conflict.
	src := map[string]interface{}{
		"tools": map[string]interface{}{
			"allow": []interface{}{"group:web"},
			"exec":  map[string]interface{}{"ask": "off"},
		},
	}
	dst := map[string]interface{}{
		"tools": map[string]interface{}{
			"allow": []interface{}{"group:web", "group:fs"},
		},
	}

	preserveHiddenOpenClawFields(dst, src)

	dstTools := dst["tools"].(map[string]interface{})
	if _, ok := dstTools["alsoAllow"]; ok {
		t.Fatal("unexpected 'alsoAllow' appeared")
	}
	// exec should be preserved from src
	if _, ok := dstTools["exec"]; !ok {
		t.Error("expected 'exec' to be preserved from src")
	}
}
