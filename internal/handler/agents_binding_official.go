package handler

import (
	"fmt"
	"sort"
	"strings"
)

var officialBindingMatchAllowedKeys = []string{
	"channel",
	"sender",
	"peer",
	"parentPeer",
	"guildId",
	"teamId",
	"accountId",
	"roles",
}

type routePreviewPeer struct {
	Kind string
	ID   string
}

type routePreviewMatch struct {
	Channel    string
	AccountID  string
	Sender     string
	Peer       *routePreviewPeer
	ParentPeer *routePreviewPeer
	GuildID    string
	TeamID     string
	Roles      []string
}

type routePreviewScope struct {
	Channel          string
	AccountID        string
	DefaultAccountID string
	Sender           string
	Peer             *routePreviewPeer
	ParentPeer       *routePreviewPeer
	GuildID          string
	TeamID           string
	Roles            map[string]struct{}
}

type routePreviewTier struct {
	matchedBy string
	label     string
	enabled   bool
	peer      *routePreviewPeer
}

func trimScalarString(v interface{}) string {
	if v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t)
	default:
		return strings.TrimSpace(fmt.Sprint(v))
	}
}

func readStrictString(v interface{}) (string, bool) {
	s, ok := v.(string)
	if !ok {
		return "", false
	}
	s = strings.TrimSpace(s)
	if s == "" {
		return "", false
	}
	return s, true
}

func readStrictStringSlice(v interface{}) ([]string, bool) {
	switch t := v.(type) {
	case []string:
		out := make([]string, 0, len(t))
		for _, item := range t {
			item = strings.TrimSpace(item)
			if item == "" {
				return nil, false
			}
			out = append(out, item)
		}
		return out, len(out) > 0
	case []interface{}:
		out := make([]string, 0, len(t))
		for _, item := range t {
			s, ok := item.(string)
			if !ok {
				return nil, false
			}
			s = strings.TrimSpace(s)
			if s == "" {
				return nil, false
			}
			out = append(out, s)
		}
		return out, len(out) > 0
	default:
		return nil, false
	}
}

func normalizePreviewPeerValue(v interface{}) *routePreviewPeer {
	if v == nil {
		return nil
	}
	switch t := v.(type) {
	case string:
		parts := strings.SplitN(strings.TrimSpace(t), ":", 2)
		if len(parts) != 2 {
			return nil
		}
		kind := strings.ToLower(strings.TrimSpace(parts[0]))
		id := strings.TrimSpace(parts[1])
		if kind == "" || id == "" {
			return nil
		}
		return &routePreviewPeer{Kind: kind, ID: id}
	case map[string]interface{}:
		kind := strings.ToLower(trimScalarString(t["kind"]))
		id := trimScalarString(t["id"])
		if kind == "" || id == "" {
			return nil
		}
		return &routePreviewPeer{Kind: kind, ID: id}
	default:
		return nil
	}
}

func normalizeBindingPeerStrict(v interface{}) (*routePreviewPeer, error) {
	obj, ok := v.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("仅支持对象")
	}
	for key := range obj {
		if key != "kind" && key != "id" {
			return nil, fmt.Errorf("对象仅允许 kind/id 字段")
		}
	}
	kind, kindOK := readStrictString(obj["kind"])
	id, idOK := readStrictString(obj["id"])
	if !kindOK || !idOK {
		return nil, fmt.Errorf("kind 和 id 不能为空")
	}
	return &routePreviewPeer{Kind: strings.ToLower(kind), ID: id}, nil
}

func normalizeBindingPeerCompat(v interface{}) (interface{}, error) {
	switch t := v.(type) {
	case string:
		peer := normalizePreviewPeerValue(t)
		if peer == nil {
			return nil, fmt.Errorf("仅支持 kind:id 字符串或 {kind,id} 对象")
		}
		return peer.Kind + ":" + peer.ID, nil
	case map[string]interface{}:
		peer, err := normalizeBindingPeerStrict(t)
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{"kind": peer.Kind, "id": peer.ID}, nil
	default:
		return nil, fmt.Errorf("仅支持 kind:id 字符串或 {kind,id} 对象")
	}
}

func bindingType(binding map[string]interface{}) string {
	if strings.EqualFold(trimScalarString(binding["type"]), "acp") {
		return "acp"
	}
	return "route"
}

func normalizeBindingTopLevel(binding map[string]interface{}) {
	if binding == nil {
		return
	}
	agentID := strings.TrimSpace(toString(binding["agentId"]))
	if agentID == "" {
		agentID = strings.TrimSpace(toString(binding["agent"]))
	}
	if agentID != "" {
		binding["agentId"] = agentID
	} else {
		delete(binding, "agentId")
	}
	delete(binding, "agent")
	if bindingType(binding) == "acp" {
		binding["type"] = "acp"
	} else {
		delete(binding, "type")
	}
	comment := trimScalarString(binding["comment"])
	if comment == "" {
		comment = trimScalarString(binding["name"])
	}
	if comment != "" {
		binding["comment"] = comment
	} else {
		delete(binding, "comment")
	}
	delete(binding, "name")
	if bindingType(binding) != "acp" {
		delete(binding, "acp")
	}
}

func buildRoutePreviewScope(meta map[string]interface{}, channelDefaultAccounts map[string]string) routePreviewScope {
	if meta == nil {
		meta = map[string]interface{}{}
	}
	channel := trimScalarString(meta["channel"])
	accountID := trimScalarString(meta["accountId"])
	defaultAccountID := strings.TrimSpace(channelDefaultAccounts[channel])
	if accountID == "" && defaultAccountID != "" {
		accountID = defaultAccountID
	}
	roleSet := map[string]struct{}{}
	for _, role := range stringSliceFromAny(meta["roles"]) {
		roleSet[role] = struct{}{}
	}
	peer := normalizePreviewPeerValue(meta["peer"])
	parentPeer := normalizePreviewPeerValue(meta["parentPeer"])
	if parentPeer == nil && peer != nil {
		parentPeer = peer
	}
	return routePreviewScope{
		Channel:          channel,
		AccountID:        accountID,
		DefaultAccountID: defaultAccountID,
		Sender:           trimScalarString(meta["sender"]),
		Peer:             peer,
		ParentPeer:       parentPeer,
		GuildID:          trimScalarString(meta["guildId"]),
		TeamID:           trimScalarString(meta["teamId"]),
		Roles:            roleSet,
	}
}

func formatRoutePreviewPeer(peer *routePreviewPeer) string {
	if peer == nil {
		return "none"
	}
	return peer.Kind + ":" + peer.ID
}

func formatRoutePreviewRoles(roles map[string]struct{}) string {
	if len(roles) == 0 {
		return "none"
	}
	list := make([]string, 0, len(roles))
	for role := range roles {
		list = append(list, role)
	}
	sort.Strings(list)
	return strings.Join(list, ",")
}

func normalizeBindingMatchForPreview(match map[string]interface{}) (routePreviewMatch, error) {
	out := routePreviewMatch{}
	if len(match) == 0 {
		return out, fmt.Errorf("empty match")
	}
	for key := range match {
		allowed := false
		for _, expected := range officialBindingMatchAllowedKeys {
			if key == expected {
				allowed = true
				break
			}
		}
		if !allowed {
			return out, fmt.Errorf("unsupported match field: %s", key)
		}
	}
	out.Channel = trimScalarString(match["channel"])
	if out.Channel == "" {
		return out, fmt.Errorf("missing match.channel")
	}
	out.AccountID = trimScalarString(match["accountId"])
	out.Sender = trimScalarString(match["sender"])
	out.GuildID = trimScalarString(match["guildId"])
	out.TeamID = trimScalarString(match["teamId"])
	if rawPeer, ok := match["peer"]; ok {
		out.Peer = normalizePreviewPeerValue(rawPeer)
		if out.Peer == nil {
			return out, fmt.Errorf("invalid match.peer")
		}
	}
	if rawParentPeer, ok := match["parentPeer"]; ok {
		out.ParentPeer = normalizePreviewPeerValue(rawParentPeer)
		if out.ParentPeer == nil {
			return out, fmt.Errorf("invalid match.parentPeer")
		}
	}
	if rawRoles, ok := match["roles"]; ok {
		roles := stringSliceFromAny(rawRoles)
		if len(roles) == 0 {
			return out, fmt.Errorf("invalid match.roles")
		}
		out.Roles = roles
	}
	if len(out.Roles) > 0 && out.GuildID == "" {
		return out, fmt.Errorf("match.roles 需与 guildId 同时使用")
	}
	return out, nil
}

func routePreviewMatchTier(match routePreviewMatch) string {
	switch {
	case match.Sender != "":
		return "binding.sender"
	case match.Peer != nil:
		return "binding.peer"
	case match.ParentPeer != nil:
		return "binding.peer.parent"
	case match.GuildID != "" && len(match.Roles) > 0:
		return "binding.guild+roles"
	case match.GuildID != "":
		return "binding.guild"
	case match.TeamID != "":
		return "binding.team"
	case match.AccountID != "" && match.AccountID != "*":
		return "binding.account"
	case match.AccountID == "*":
		return "binding.account.wildcard"
	default:
		return "binding.channel"
	}
}

func routePreviewTierMatchesBinding(tier string, match routePreviewMatch) bool {
	return routePreviewMatchTier(match) == tier
}

func routePreviewPeerKindEquals(expected, actual string) bool {
	expected = strings.ToLower(strings.TrimSpace(expected))
	actual = strings.ToLower(strings.TrimSpace(actual))
	if expected == "" || actual == "" {
		return false
	}
	if expected == "*" || actual == "*" {
		return true
	}
	if expected == actual {
		return true
	}
	return (expected == "group" && actual == "channel") || (expected == "channel" && actual == "group")
}

func routePreviewPeerEquals(expected, actual *routePreviewPeer) bool {
	if expected == nil || actual == nil {
		return false
	}
	if !routePreviewPeerKindEquals(expected.Kind, actual.Kind) {
		return false
	}
	if expected.ID == "*" {
		return true
	}
	return expected.ID == actual.ID
}

func routePreviewBindingMatches(scope routePreviewScope, match routePreviewMatch) bool {
	if scope.Channel == "" || match.Channel != scope.Channel {
		return false
	}
	switch match.AccountID {
	case "":
		if scope.DefaultAccountID != "" && scope.AccountID != scope.DefaultAccountID {
			return false
		}
		if scope.DefaultAccountID == "" && scope.AccountID != "" {
			return false
		}
	case "*":
		// Channel-wide rule.
	default:
		if scope.AccountID != match.AccountID {
			return false
		}
	}
	if match.Sender != "" && scope.Sender != match.Sender {
		return false
	}
	if match.Peer != nil && !routePreviewPeerEquals(match.Peer, scope.Peer) {
		return false
	}
	if match.ParentPeer != nil && !routePreviewPeerEquals(match.ParentPeer, scope.ParentPeer) {
		return false
	}
	if match.GuildID != "" && scope.GuildID != match.GuildID {
		return false
	}
	if match.TeamID != "" && scope.TeamID != match.TeamID {
		return false
	}
	if len(match.Roles) > 0 {
		hit := false
		for _, role := range match.Roles {
			if _, ok := scope.Roles[role]; ok {
				hit = true
				break
			}
		}
		if !hit {
			return false
		}
	}
	return true
}

func buildRoutePreviewTiers(scope routePreviewScope) []routePreviewTier {
	return []routePreviewTier{
		{matchedBy: "binding.sender", label: "sender", enabled: scope.Sender != ""},
		{matchedBy: "binding.peer", label: "peer", enabled: scope.Peer != nil, peer: scope.Peer},
		{matchedBy: "binding.peer.parent", label: "parent peer", enabled: scope.ParentPeer != nil, peer: scope.ParentPeer},
		{matchedBy: "binding.guild+roles", label: "guild+roles", enabled: scope.GuildID != "" && len(scope.Roles) > 0},
		{matchedBy: "binding.guild", label: "guild", enabled: scope.GuildID != ""},
		{matchedBy: "binding.team", label: "team", enabled: scope.TeamID != ""},
		{matchedBy: "binding.account", label: "account", enabled: scope.AccountID != ""},
		{matchedBy: "binding.account.wildcard", label: "account wildcard", enabled: scope.AccountID != ""},
		{matchedBy: "binding.channel", label: "channel", enabled: scope.Channel != ""},
	}
}

func evaluateOfficialRoutePreview(meta map[string]interface{}, bindings []map[string]interface{}, defaultAgent string, channelDefaultAccounts map[string]string) (string, string, int, []string) {
	if isLegacySingleAgentMode() {
		return "main", "legacy-single-agent", -1, []string{"LEGACY_SINGLE_AGENT=true", "fallback main"}
	}
	scope := buildRoutePreviewScope(meta, channelDefaultAccounts)
	trace := []string{
		fmt.Sprintf(
			"scope channel=%s account=%s defaultAccount=%s sender=%s peer=%s parentPeer=%s guild=%s team=%s roles=%s",
			scope.Channel,
			scope.AccountID,
			scope.DefaultAccountID,
			scope.Sender,
			formatRoutePreviewPeer(scope.Peer),
			formatRoutePreviewPeer(scope.ParentPeer),
			scope.GuildID,
			scope.TeamID,
			formatRoutePreviewRoles(scope.Roles),
		),
	}
	for _, tier := range buildRoutePreviewTiers(scope) {
		if !tier.enabled {
			trace = append(trace, fmt.Sprintf("skip %s: missing scope", tier.matchedBy))
			continue
		}
		for index, raw := range bindings {
			binding := deepCloneMap(raw)
			if enabled, ok := raw["enabled"].(bool); ok && !enabled {
				trace = append(trace, fmt.Sprintf("skip bindings[%d]: disabled", index))
				continue
			}
			normalizeBindingTopLevel(binding)
			if bindingType(binding) != "route" {
				continue
			}
			agentID := extractBindingAgentID(binding)
			if agentID == "" {
				trace = append(trace, fmt.Sprintf("skip bindings[%d]: missing agentId", index))
				continue
			}
			matchRaw, ok := binding["match"].(map[string]interface{})
			if !ok || len(matchRaw) == 0 {
				trace = append(trace, fmt.Sprintf("skip bindings[%d]: missing match", index))
				continue
			}
			match, err := normalizeBindingMatchForPreview(matchRaw)
			if err != nil {
				trace = append(trace, fmt.Sprintf("skip bindings[%d]: %s", index, err.Error()))
				continue
			}
			if !routePreviewTierMatchesBinding(tier.matchedBy, match) {
				continue
			}
			if !routePreviewBindingMatches(scope, match) {
				continue
			}
			trace = append(trace, fmt.Sprintf("select bindings[%d]: %s", index, tier.label))
			return agentID, tier.matchedBy, index, trace
		}
		trace = append(trace, fmt.Sprintf("tier %s: no match", tier.matchedBy))
	}
	if strings.TrimSpace(defaultAgent) == "" {
		defaultAgent = "main"
	}
	trace = append(trace, fmt.Sprintf("fallback default: %s", defaultAgent))
	return defaultAgent, "default", -1, trace
}

func validateBindingMatchForWrite(index int, match map[string]interface{}) error {
	if len(match) == 0 {
		return fmt.Errorf("bindings[%d] 缺少有效 match", index)
	}
	channel := trimScalarString(match["channel"])
	if channel == "" {
		return fmt.Errorf("bindings[%d].match.channel 必填", index)
	}
	for key, val := range match {
		allowed := false
		for _, expected := range officialBindingMatchAllowedKeys {
			if key == expected {
				allowed = true
				break
			}
		}
		if !allowed {
			return fmt.Errorf("bindings[%d].match.%s 不支持，允许字段: %s", index, key, strings.Join(officialBindingMatchAllowedKeys, ", "))
		}
		switch key {
		case "channel", "sender", "guildId", "teamId", "accountId":
			if _, ok := readStrictString(val); !ok {
				return fmt.Errorf("bindings[%d].match.%s 仅支持非空字符串", index, key)
			}
		case "peer", "parentPeer":
			if _, err := normalizeBindingPeerCompat(val); err != nil {
				return fmt.Errorf("bindings[%d].match.%s %s", index, key, err.Error())
			}
		case "roles":
			if _, ok := readStrictStringSlice(val); !ok {
				return fmt.Errorf("bindings[%d].match.roles 仅支持字符串数组", index)
			}
		}
	}
	if _, ok := match["roles"]; ok {
		if trimScalarString(match["guildId"]) == "" {
			return fmt.Errorf("bindings[%d].match.roles 需与 guildId 同时使用", index)
		}
	}
	return nil
}

func validateAcpBindingConfig(index int, raw interface{}) error {
	if raw == nil {
		return nil
	}
	obj, ok := raw.(map[string]interface{})
	if !ok {
		return fmt.Errorf("bindings[%d].acp 必须是对象", index)
	}
	for key, val := range obj {
		switch key {
		case "mode":
			mode := trimScalarString(val)
			if mode == "" || (mode != "persistent" && mode != "oneshot") {
				return fmt.Errorf("bindings[%d].acp.mode 仅支持 persistent/oneshot", index)
			}
		case "label", "cwd", "backend":
			if trimScalarString(val) == "" {
				return fmt.Errorf("bindings[%d].acp.%s 不能为空字符串", index, key)
			}
		default:
			return fmt.Errorf("bindings[%d].acp.%s 不支持", index, key)
		}
	}
	return nil
}

func validateBindingForWrite(index int, binding map[string]interface{}, agentSet map[string]struct{}) error {
	normalizeBindingTopLevel(binding)
	agentID := extractBindingAgentID(binding)
	if agentID == "" {
		return fmt.Errorf("bindings[%d] 缺少 agentId", index)
	}
	if _, ok := agentSet[agentID]; !ok {
		return fmt.Errorf("bindings[%d] 指向不存在的 agent: %s", index, agentID)
	}
	rawType := trimScalarString(binding["type"])
	if rawType != "" && rawType != "acp" && rawType != "route" {
		return fmt.Errorf("bindings[%d].type 仅支持 route/acp", index)
	}
	if enabled, ok := binding["enabled"]; ok {
		if _, ok := enabled.(bool); !ok {
			return fmt.Errorf("bindings[%d].enabled 必须是布尔值", index)
		}
	}
	if comment, ok := binding["comment"]; ok && trimScalarString(comment) == "" {
		delete(binding, "comment")
	}
	match, ok := binding["match"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("bindings[%d] 缺少有效 match", index)
	}
	if err := validateBindingMatchForWrite(index, match); err != nil {
		return err
	}
	if bindingType(binding) == "acp" {
		return validateAcpBindingConfig(index, binding["acp"])
	}
	if _, ok := binding["acp"]; ok {
		return fmt.Errorf("bindings[%d].acp 仅能在 type=acp 时使用", index)
	}
	return nil
}

func validateBindingsUniqueAccounts(bindings []map[string]interface{}) error {
	type owner struct {
		agentID string
		index   int
	}
	owners := map[string][]owner{}
	for i, binding := range bindings {
		if binding == nil {
			continue
		}
		if enabled, ok := binding["enabled"].(bool); ok && !enabled {
			continue
		}
		if bindingType(binding) != "route" {
			continue
		}
		match, _ := binding["match"].(map[string]interface{})
		channel := trimScalarString(match["channel"])
		accountID := trimScalarString(match["accountId"])
		if channel == "" || accountID == "" || accountID == "*" {
			continue
		}
		agentID := extractBindingAgentID(binding)
		key := channel + "\x00" + accountID
		owners[key] = append(owners[key], owner{agentID: agentID, index: i})
	}
	for key, items := range owners {
		uniq := map[string]struct{}{}
		labels := make([]string, 0, len(items))
		for _, item := range items {
			if _, ok := uniq[item.agentID]; ok {
				continue
			}
			uniq[item.agentID] = struct{}{}
			labels = append(labels, fmt.Sprintf("%s(bindings[%d])", item.agentID, item.index))
		}
		if len(labels) > 1 {
			parts := strings.SplitN(key, "\x00", 2)
			return fmt.Errorf("检测到重复账号路由：channel=%s accountId=%s 同时指向多个智能体：%s；请保证一账号只对应一个智能体", parts[0], parts[1], strings.Join(labels, ", "))
		}
	}
	return nil
}

func normalizeBindingMatchForWrite(raw map[string]interface{}) map[string]interface{} {
	match := map[string]interface{}{}
	if raw == nil {
		return match
	}
	if channel := trimScalarString(raw["channel"]); channel != "" {
		match["channel"] = channel
	}
	if accountID := trimScalarString(raw["accountId"]); accountID != "" {
		match["accountId"] = accountID
	}
	if sender := trimScalarString(raw["sender"]); sender != "" {
		match["sender"] = sender
	}
	if guildID := trimScalarString(raw["guildId"]); guildID != "" {
		match["guildId"] = guildID
	}
	if teamID := trimScalarString(raw["teamId"]); teamID != "" {
		match["teamId"] = teamID
	}
	if rawPeer, ok := raw["peer"]; ok {
		if peer, err := normalizeBindingPeerCompat(rawPeer); err == nil && peer != nil {
			match["peer"] = peer
		}
	}
	if rawParentPeer, ok := raw["parentPeer"]; ok {
		if peer, err := normalizeBindingPeerCompat(rawParentPeer); err == nil && peer != nil {
			match["parentPeer"] = peer
		}
	}
	if roles := stringSliceFromAny(raw["roles"]); len(roles) > 0 {
		match["roles"] = roles
	}
	return match
}

func normalizeAcpBindingForWrite(raw interface{}) map[string]interface{} {
	obj, ok := raw.(map[string]interface{})
	if !ok {
		return nil
	}
	acp := map[string]interface{}{}
	if mode := trimScalarString(obj["mode"]); mode != "" {
		acp["mode"] = mode
	}
	if label := trimScalarString(obj["label"]); label != "" {
		acp["label"] = label
	}
	if cwd := trimScalarString(obj["cwd"]); cwd != "" {
		acp["cwd"] = cwd
	}
	if backend := trimScalarString(obj["backend"]); backend != "" {
		acp["backend"] = backend
	}
	if len(acp) == 0 {
		return nil
	}
	return acp
}

func normalizeBindingForWrite(binding map[string]interface{}) map[string]interface{} {
	normalizeBindingTopLevel(binding)
	out := map[string]interface{}{}
	if bindingType(binding) == "acp" {
		out["type"] = "acp"
	}
	if agentID := extractBindingAgentID(binding); agentID != "" {
		out["agentId"] = agentID
	}
	if comment := trimScalarString(binding["comment"]); comment != "" {
		out["comment"] = comment
	}
	if enabled, ok := binding["enabled"].(bool); ok && !enabled {
		out["enabled"] = false
	}
	if matchRaw, ok := binding["match"].(map[string]interface{}); ok {
		out["match"] = normalizeBindingMatchForWrite(matchRaw)
	}
	if bindingType(binding) == "acp" {
		if acp := normalizeAcpBindingForWrite(binding["acp"]); len(acp) > 0 {
			out["acp"] = acp
		}
	}
	return out
}
