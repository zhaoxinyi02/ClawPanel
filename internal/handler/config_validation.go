package handler

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

type numericFieldConstraint struct {
	label   string
	integer bool
	min     *float64
	max     *float64
}

func float64Ptr(v float64) *float64 {
	return &v
}

func formatNumericBound(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}

func parseNumericField(raw interface{}) (float64, bool) {
	switch v := raw.(type) {
	case int:
		return float64(v), true
	case int32:
		return float64(v), true
	case int64:
		return float64(v), true
	case float32:
		return float64(v), true
	case float64:
		return v, true
	case string:
		trimmed := strings.TrimSpace(v)
		if trimmed == "" {
			return 0, false
		}
		parsed, err := strconv.ParseFloat(trimmed, 64)
		if err != nil {
			return 0, false
		}
		return parsed, true
	default:
		return 0, false
	}
}

func validateNumericField(raw interface{}, constraint numericFieldConstraint) error {
	if raw == nil {
		return nil
	}
	value, ok := parseNumericField(raw)
	if !ok || !isFiniteNumber(value) {
		return fmt.Errorf("%s 必须是数字", constraint.label)
	}
	if constraint.integer && value != math.Trunc(value) {
		return fmt.Errorf("%s 必须是整数", constraint.label)
	}
	if constraint.min != nil && value < *constraint.min {
		return fmt.Errorf("%s 不能小于 %s", constraint.label, formatNumericBound(*constraint.min))
	}
	if constraint.max != nil && value > *constraint.max {
		return fmt.Errorf("%s 不能大于 %s", constraint.label, formatNumericBound(*constraint.max))
	}
	return nil
}

func validateEnumField(raw interface{}, label string, allowed ...string) error {
	if raw == nil {
		return nil
	}
	value := strings.TrimSpace(toString(raw))
	if value == "" {
		return nil
	}
	for _, item := range allowed {
		if value == item {
			return nil
		}
	}
	return fmt.Errorf("%s 必须是以下值之一: %s", label, strings.Join(allowed, ", "))
}

func isFiniteNumber(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0)
}

func getNestedMapValue(root map[string]interface{}, path ...string) interface{} {
	current := interface{}(root)
	for _, key := range path {
		node, ok := current.(map[string]interface{})
		if !ok || node == nil {
			return nil
		}
		current = node[key]
	}
	return current
}

func validateOpenClawNumericConfig(ocCfg map[string]interface{}) error {
	if ocCfg == nil {
		return nil
	}
	rules := []struct {
		value      interface{}
		constraint numericFieldConstraint
	}{
		{
			value: getNestedMapValue(ocCfg, "agents", "defaults", "contextTokens"),
			constraint: numericFieldConstraint{
				label:   "agents.defaults.contextTokens",
				integer: true,
				min:     float64Ptr(1),
			},
		},
		{
			value: getNestedMapValue(ocCfg, "agents", "defaults", "maxConcurrent"),
			constraint: numericFieldConstraint{
				label:   "agents.defaults.maxConcurrent",
				integer: true,
				min:     float64Ptr(1),
			},
		},
		{
			value: getNestedMapValue(ocCfg, "agents", "defaults", "compaction", "maxHistoryShare"),
			constraint: numericFieldConstraint{
				label: "agents.defaults.compaction.maxHistoryShare",
				min:   float64Ptr(0),
				max:   float64Ptr(1),
			},
		},
		{
			value: getNestedMapValue(ocCfg, "session", "maintenance", "maxEntries"),
			constraint: numericFieldConstraint{
				label:   "session.maintenance.maxEntries",
				integer: true,
				min:     float64Ptr(1),
			},
		},
		{
			value: getNestedMapValue(ocCfg, "session", "agentToAgent", "maxPingPongTurns"),
			constraint: numericFieldConstraint{
				label:   "session.agentToAgent.maxPingPongTurns",
				integer: true,
				min:     float64Ptr(1),
			},
		},
		{
			value: getNestedMapValue(ocCfg, "gateway", "port"),
			constraint: numericFieldConstraint{
				label:   "gateway.port",
				integer: true,
				min:     float64Ptr(1),
				max:     float64Ptr(65535),
			},
		},
	}
	for _, rule := range rules {
		if err := validateNumericField(rule.value, rule.constraint); err != nil {
			return err
		}
	}
	if err := validateEnumField(getNestedMapValue(ocCfg, "session", "dmScope"), "session.dmScope",
		"main", "per-peer", "per-channel-peer", "per-account-channel-peer",
	); err != nil {
		return err
	}
	return nil
}

func validateAgentContextConfig(agent map[string]interface{}) error {
	if agent == nil {
		return nil
	}
	if err := validateNumericField(agent["contextTokens"], numericFieldConstraint{
		label:   "contextTokens",
		integer: true,
		min:     float64Ptr(1),
	}); err != nil {
		return err
	}
	compaction, _ := agent["compaction"].(map[string]interface{})
	if err := validateNumericField(getNestedMapValue(map[string]interface{}{"compaction": compaction}, "compaction", "maxHistoryShare"), numericFieldConstraint{
		label: "compaction.maxHistoryShare",
		min:   float64Ptr(0),
		max:   float64Ptr(1),
	}); err != nil {
		return err
	}
	return nil
}
