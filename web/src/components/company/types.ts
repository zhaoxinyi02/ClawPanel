export type CompanyAgentOption = {
  id: string;
  name: string;
  isDefault?: boolean;
};

export type CompanyTeamMember = {
  id?: number;
  teamId?: string;
  agentId: string;
  agentName?: string;
  roleType?: string;
  dutyLabel?: string;
  sortOrder?: number;
  enabled?: boolean;
};

export type CompanyTeamRecord = {
  id: string;
  name: string;
  description?: string;
  managerAgentId?: string;
  defaultSummaryAgentId?: string;
  status?: string;
  agents?: CompanyTeamMember[];
};

export type CompanyTaskStepRecord = {
  id: string;
  taskId: string;
  stepKey: string;
  title: string;
  instruction?: string;
  workerAgentId?: string;
  status: string;
  orderIndex: number;
  input?: Record<string, unknown>;
  outputText?: string;
  errorText?: string;
  createdAt?: number;
  updatedAt?: number;
};

export type CompanyEventRecord = {
  id: number;
  taskId: string;
  stepId?: string;
  eventType: string;
  message: string;
  createdAt: number;
};

export type CompanyTaskRecord = {
  id: string;
  teamId?: string;
  title: string;
  goal: string;
  status: string;
  managerAgentId?: string;
  summaryAgentId?: string;
  sourceType?: string;
  sourceChannelType?: string;
  sourceChannelId?: string;
  sourceRefId?: string;
  deliveryType?: string;
  deliveryChannelType?: string;
  deliveryChannelId?: string;
  deliveryTargetId?: string;
  panelSessionId?: string;
  resultText?: string;
  reviewResult?: string;
  reviewComment?: string;
  createdAt: number;
  updatedAt: number;
  steps?: CompanyTaskStepRecord[];
  events?: CompanyEventRecord[];
};

export type CompanyOverviewData = {
  teamCount: number;
  taskCount: number;
  runningCount: number;
  completedCount: number;
  recentTasks: CompanyTaskRecord[];
};

export type CompanyBindingRecord = {
  agentId: string;
  type?: string;
  enabled?: boolean;
  comment?: string;
  match?: Record<string, unknown>;
  acp?: Record<string, unknown>;
};

export type CompanyChannelConfigMeta = {
  id: string;
  label: string;
  accounts: string[];
  defaultAccount?: string;
  enabled: boolean;
};

export type CompanyAgentAccessChannelSummary = {
  id: string;
  label: string;
  routeCount: number;
  explicitAccounts: string[];
  defaultAccount?: string;
  configuredAccounts: number;
  enabled: boolean;
};

export type CompanyAgentAccessSummary = {
  agentId: string;
  routeCount: number;
  channelCount: number;
  channels: CompanyAgentAccessChannelSummary[];
  defaultFallback: boolean;
  hasCoverage: boolean;
};

export type CompanyTeamAccessSummary = {
  memberCount: number;
  coveredMemberCount: number;
  workerCoveredCount: number;
  totalRouteCount: number;
  channelLabels: string[];
};

export type CompanyBindingDuplicateWarning = {
  channelId: string;
  channelLabel: string;
  accountId: string;
  agentIds: string[];
};

export type NormalizedCompanyTeam = CompanyTeamRecord & {
  manager: CompanyTeamMember | null;
  workers: CompanyTeamMember[];
  memberCount: number;
  summaryAgentId: string;
  managerDerived: boolean;
};

const COMPANY_MANAGER_NAME_HINTS = [
  'ai company manager',
  'company manager',
  'ai公司manager',
  'ai公司主管',
  '公司主管',
  '公司经理',
];

export function asString(value: unknown) {
  if (typeof value === 'string') return value.trim();
  if (typeof value === 'number') return String(value);
  return '';
}

export function isRecord(value: unknown): value is Record<string, unknown> {
  return !!value && typeof value === 'object' && !Array.isArray(value);
}

export function extractTextMatchValue(value: unknown) {
  if (typeof value === 'string') return value.trim();
  if (typeof value === 'number') return String(value);
  return '';
}

const CHANNEL_LABELS: Record<string, string> = {
  qq: 'QQ 个人号',
  qqbot: 'QQ 官方机器人',
  telegram: 'Telegram',
  discord: 'Discord',
  feishu: '飞书',
  wecom: '企业微信',
  'wecom-app': '企业微信应用',
  wechat: '微信',
  api: 'API',
  webhook: 'Webhook',
  panel_chat: '面板聊天',
  panel_manual: '面板手工创建',
};

export function humanizeCompanyChannelName(channelId: string) {
  const normalized = asString(channelId);
  return CHANNEL_LABELS[normalized] || normalized || '未命名通道';
}

function normalizeName(agent: CompanyAgentOption | CompanyTeamMember | null | undefined) {
  if (!agent) return '';
  return asString('name' in agent ? agent.name : agent.agentName) || asString('id' in agent ? agent.id : agent.agentId);
}

export function isPreferredCompanyManager(agent: CompanyAgentOption | CompanyTeamMember | null | undefined) {
  if (!agent) return false;
  const id = asString('id' in agent ? agent.id : agent.agentId).toLowerCase();
  const name = normalizeName(agent).toLowerCase();
  if (id === 'company_manager' || id.startsWith('company_manager')) return true;
  return COMPANY_MANAGER_NAME_HINTS.some(keyword => name.includes(keyword));
}

export function buildCompanyAgentOptions(raw: unknown): CompanyAgentOption[] {
  if (!raw || !Array.isArray(raw)) return [];
  const seen = new Set<string>();
  const items: CompanyAgentOption[] = [];
  for (const entry of raw) {
    if (!entry || typeof entry !== 'object') continue;
    const record = entry as Record<string, unknown>;
    const id = asString(record.id);
    if (!id || seen.has(id)) continue;
    seen.add(id);
    const identity = isRecord(record.identity) ? record.identity : {};
    items.push({
      id,
      name: asString(record.name) || asString(identity.name) || id,
      isDefault: Boolean(record.default),
    });
  }
  return items;
}

export function buildCompanyChannelMeta(raw: unknown): Record<string, CompanyChannelConfigMeta> {
  if (!isRecord(raw)) return {};
  const out: Record<string, CompanyChannelConfigMeta> = {};
  for (const [channelId, cfgRaw] of Object.entries(raw)) {
    const id = asString(channelId);
    if (!id) continue;
    const cfg = isRecord(cfgRaw) ? cfgRaw : {};
    const accountsObj = isRecord(cfg.accounts) ? cfg.accounts : {};
    const accounts = Object.keys(accountsObj).map(item => item.trim()).filter(Boolean).sort();
    let defaultAccount = asString(cfg.defaultAccount);
    if (!defaultAccount) {
      if (accounts.includes('default')) defaultAccount = 'default';
      else if (accounts.length > 0) defaultAccount = accounts[0];
    }
    out[id] = {
      id,
      label: humanizeCompanyChannelName(id),
      accounts,
      defaultAccount: defaultAccount || undefined,
      enabled: cfg.enabled !== false,
    };
  }
  return out;
}

export function buildCompanyBindingRecords(raw: unknown): CompanyBindingRecord[] {
  if (!Array.isArray(raw)) return [];
  const items: CompanyBindingRecord[] = [];
  for (const entry of raw) {
    if (!isRecord(entry)) continue;
    const agentId = asString(entry.agentId);
    if (!agentId) continue;
    items.push({
      agentId,
      type: asString(entry.type) || 'route',
      enabled: entry.enabled !== false,
      comment: asString(entry.comment),
      match: isRecord(entry.match) ? entry.match : {},
      acp: isRecord(entry.acp) ? entry.acp : undefined,
    });
  }
  return items;
}

export function serializeCompanyBindingRecords(bindings: CompanyBindingRecord[]) {
  return bindings.map(binding => {
    const payload: Record<string, unknown> = {
      agentId: asString(binding.agentId),
      comment: asString(binding.comment) || undefined,
      match: isRecord(binding.match) ? binding.match : {},
    };
    if (binding.enabled === false) payload.enabled = false;
    if (asString(binding.type) && asString(binding.type) !== 'route') payload.type = asString(binding.type);
    if (isRecord(binding.acp) && Object.keys(binding.acp).length > 0) payload.acp = binding.acp;
    return payload;
  });
}

export function retargetAgentChannelBindingAccount(
  bindings: CompanyBindingRecord[],
  agentId: string,
  channelId: string,
  nextAccountId: string,
) {
  const normalizedAgentId = asString(agentId);
  const normalizedChannelId = asString(channelId);
  const normalizedAccountId = asString(nextAccountId);
  let changed = false;
  let matchedBinding = false;
  const nextBindings = bindings.map(binding => {
    if (asString(binding.agentId) !== normalizedAgentId) return binding;
    const match = isRecord(binding.match) ? { ...binding.match } : {};
    if (extractTextMatchValue(match.channel) !== normalizedChannelId) return binding;
    matchedBinding = true;
    changed = true;
    if (normalizedAccountId) match.accountId = normalizedAccountId;
    else delete match.accountId;
    return {
      ...binding,
      match,
    };
  });
  if (!matchedBinding && normalizedAgentId && normalizedChannelId) {
    changed = true;
    nextBindings.push({
      agentId: normalizedAgentId,
      enabled: true,
      comment: `${humanizeCompanyChannelName(normalizedChannelId)} 显式绑定`,
      match: normalizedAccountId
        ? { channel: normalizedChannelId, accountId: normalizedAccountId }
        : { channel: normalizedChannelId },
    });
  }
  return { nextBindings, changed };
}

export function buildCompanyAgentAccessMap(
  agentIds: string[],
  bindingsRaw: unknown,
  channelsRaw: unknown,
  defaultAgentId?: string,
): Record<string, CompanyAgentAccessSummary> {
  const channelMeta = buildCompanyChannelMeta(channelsRaw);
  const bindings = buildCompanyBindingRecords(bindingsRaw);
  const out: Record<string, CompanyAgentAccessSummary> = {};
  for (const agentId of agentIds.map(item => asString(item)).filter(Boolean)) {
    out[agentId] = {
      agentId,
      routeCount: 0,
      channelCount: 0,
      channels: [],
      defaultFallback: agentId === asString(defaultAgentId),
      hasCoverage: agentId === asString(defaultAgentId),
    };
  }

  const perAgentChannels: Record<string, Map<string, CompanyAgentAccessChannelSummary>> = {};
  for (const binding of bindings) {
    if (binding.enabled === false) continue;
    const agentId = asString(binding.agentId);
    if (!agentId) continue;
    if (!out[agentId]) {
      out[agentId] = {
        agentId,
        routeCount: 0,
        channelCount: 0,
        channels: [],
        defaultFallback: agentId === asString(defaultAgentId),
        hasCoverage: agentId === asString(defaultAgentId),
      };
    }
    out[agentId].routeCount += 1;
    out[agentId].hasCoverage = true;

    const match = isRecord(binding.match) ? binding.match : {};
    const channelId = extractTextMatchValue(match.channel);
    if (!channelId) continue;
    if (!perAgentChannels[agentId]) perAgentChannels[agentId] = new Map<string, CompanyAgentAccessChannelSummary>();
    const current = perAgentChannels[agentId].get(channelId) || {
      id: channelId,
      label: channelMeta[channelId]?.label || humanizeCompanyChannelName(channelId),
      routeCount: 0,
      explicitAccounts: [],
      defaultAccount: channelMeta[channelId]?.defaultAccount,
      configuredAccounts: channelMeta[channelId]?.accounts.length || 0,
      enabled: channelMeta[channelId]?.enabled ?? true,
    };
    current.routeCount += 1;
    const accountId = extractTextMatchValue(match.accountId);
    if (accountId && !current.explicitAccounts.includes(accountId)) {
      current.explicitAccounts.push(accountId === '*' ? '全部账号' : accountId);
    }
    perAgentChannels[agentId].set(channelId, current);
  }

  for (const [agentId, summary] of Object.entries(out)) {
    const channels = Array.from(perAgentChannels[agentId]?.values() || []).sort((left, right) => {
      if (left.routeCount !== right.routeCount) return right.routeCount - left.routeCount;
      return left.label.localeCompare(right.label);
    });
    summary.channels = channels;
    summary.channelCount = channels.length;
    summary.hasCoverage = summary.hasCoverage || summary.routeCount > 0;
  }

  return out;
}

export function summarizeCompanyTeamAccess(team: NormalizedCompanyTeam | null, accessMap: Record<string, CompanyAgentAccessSummary>) {
  if (!team) {
    return { memberCount: 0, coveredMemberCount: 0, workerCoveredCount: 0, totalRouteCount: 0, channelLabels: [] } satisfies CompanyTeamAccessSummary;
  }
  const members = [team.manager, ...team.workers].filter(Boolean) as CompanyTeamMember[];
  const coveredMembers = members.filter(member => accessMap[member.agentId]?.hasCoverage);
  const workerCovered = team.workers.filter(worker => accessMap[worker.agentId]?.hasCoverage);
  const totalRouteCount = members.reduce((sum, member) => sum + (accessMap[member.agentId]?.routeCount || 0), 0);
  const channelLabels = Array.from(new Set(members.flatMap(member => (accessMap[member.agentId]?.channels || []).map(channel => channel.label))));
  return {
    memberCount: members.length,
    coveredMemberCount: coveredMembers.length,
    workerCoveredCount: workerCovered.length,
    totalRouteCount,
    channelLabels,
  } satisfies CompanyTeamAccessSummary;
}

export function countConfiguredCompanyChannels(raw: unknown) {
  return Object.values(buildCompanyChannelMeta(raw)).filter(channel => channel.enabled).length;
}

export function detectCompanyBindingDuplicates(bindingsRaw: unknown, channelsRaw?: unknown) {
  const channelMeta = buildCompanyChannelMeta(channelsRaw);
  const bindings = buildCompanyBindingRecords(bindingsRaw);
  const owners = new Map<string, Set<string>>();
  for (const binding of bindings) {
    if (binding.enabled === false) continue;
    if (asString(binding.type) && asString(binding.type) !== 'route') continue;
    const match = isRecord(binding.match) ? binding.match : {};
    const channelId = extractTextMatchValue(match.channel);
    const accountId = extractTextMatchValue(match.accountId);
    const agentId = asString(binding.agentId);
    if (!channelId || !accountId || accountId === '*' || !agentId) continue;
    const key = `${channelId}\u0000${accountId}`;
    if (!owners.has(key)) owners.set(key, new Set<string>());
    owners.get(key)?.add(agentId);
  }
  const warnings: CompanyBindingDuplicateWarning[] = [];
  for (const [key, agentIDs] of owners.entries()) {
    if (agentIDs.size <= 1) continue;
    const [channelId, accountId] = key.split('\u0000');
    warnings.push({
      channelId,
      channelLabel: channelMeta[channelId]?.label || humanizeCompanyChannelName(channelId),
      accountId,
      agentIds: Array.from(agentIDs).sort(),
    });
  }
  warnings.sort((left, right) => {
    if (left.channelLabel !== right.channelLabel) return left.channelLabel.localeCompare(right.channelLabel);
    return left.accountId.localeCompare(right.accountId);
  });
  return warnings;
}

export function findAgentOption(agents: CompanyAgentOption[], agentId?: string) {
  const id = asString(agentId);
  return agents.find(agent => agent.id === id) || null;
}

export function pickPreferredCompanyManager(agents: CompanyAgentOption[], fallbackId?: string) {
  const exact = agents.find(agent => agent.id === 'company_manager') || null;
  if (exact) return exact;
  const prefixed = agents.find(agent => agent.id.startsWith('company_manager')) || null;
  if (prefixed) return prefixed;
  const named = agents.find(agent => isPreferredCompanyManager(agent)) || null;
  if (named) return named;
  const fallback = findAgentOption(agents, fallbackId);
  if (fallback) return fallback;
  const byDefault = agents.find(agent => agent.isDefault) || null;
  return byDefault || agents[0] || null;
}

export function agentLabel(agent: CompanyAgentOption | CompanyTeamMember | null | undefined) {
  if (!agent) return '';
  const id = asString('id' in agent ? agent.id : agent.agentId);
  const name = normalizeName(agent);
  if (!name || name === id) return id;
  return `${name} (${id})`;
}

function normalizeMember(
  member: CompanyTeamMember,
  agents: CompanyAgentOption[],
  fallbackRole: 'manager' | 'worker',
): CompanyTeamMember {
  const option = findAgentOption(agents, member.agentId);
  return {
    ...member,
    agentId: asString(member.agentId),
    agentName: member.agentName || option?.name || member.agentId,
    roleType: member.roleType || fallbackRole,
    dutyLabel: member.dutyLabel || (fallbackRole === 'manager' ? '调度 / 汇总' : '执行'),
    enabled: member.enabled ?? true,
  };
}

export function normalizeCompanyTeam(team: CompanyTeamRecord, agents: CompanyAgentOption[]): NormalizedCompanyTeam {
  const inputAgents = Array.isArray(team.agents) ? team.agents : [];
  const preferredManager = pickPreferredCompanyManager(agents, team.managerAgentId);
  const explicitManagerId = asString(team.managerAgentId);
  const legacyManager = inputAgents.find(agent => asString(agent.roleType).toLowerCase() === 'manager');
  const managerId = explicitManagerId || legacyManager?.agentId || preferredManager?.id || 'main';
  const managerDerived = !explicitManagerId && (!legacyManager || legacyManager.agentId !== managerId);

  const seen = new Set<string>();
  const workers: CompanyTeamMember[] = [];
  for (const rawMember of inputAgents) {
    const agentId = asString(rawMember.agentId);
    if (!agentId || seen.has(agentId) || agentId === managerId) continue;
    seen.add(agentId);
    workers.push(normalizeMember({ ...rawMember, roleType: 'worker' }, agents, 'worker'));
  }

  const managerSource = legacyManager && asString(legacyManager.agentId) === managerId
    ? legacyManager
    : { agentId: managerId, roleType: 'manager', enabled: true };
  const manager = normalizeMember(managerSource, agents, 'manager');

  workers.sort((a, b) => {
    const left = a.sortOrder ?? Number.MAX_SAFE_INTEGER;
    const right = b.sortOrder ?? Number.MAX_SAFE_INTEGER;
    if (left !== right) return left - right;
    return agentLabel(a).localeCompare(agentLabel(b));
  });

  return {
    ...team,
    managerAgentId: managerId,
    defaultSummaryAgentId: asString(team.defaultSummaryAgentId) || managerId,
    manager,
    workers,
    memberCount: workers.length + (manager ? 1 : 0),
    summaryAgentId: asString(team.defaultSummaryAgentId) || managerId,
    managerDerived,
    agents: manager ? [manager, ...workers] : workers,
  };
}

export function buildTaskFallbackTeam(task: CompanyTaskRecord, agents: CompanyAgentOption[]): NormalizedCompanyTeam {
  const workerIds = Array.from(new Set((task.steps || []).map(step => asString(step.workerAgentId)).filter(Boolean)));
  return normalizeCompanyTeam({
    id: task.teamId || task.id,
    name: '任务协作关系',
    description: '基于任务参与者自动推导',
    managerAgentId: task.managerAgentId,
    defaultSummaryAgentId: task.summaryAgentId || task.managerAgentId,
    status: task.status,
    agents: [
      ...(task.managerAgentId ? [{ agentId: task.managerAgentId, roleType: 'manager', dutyLabel: '调度 / 汇总', enabled: true }] : []),
      ...workerIds.map((workerAgentId, index) => ({ agentId: workerAgentId, roleType: 'worker', dutyLabel: `执行步骤 ${index + 1}`, sortOrder: index, enabled: true })),
    ],
  }, agents);
}

export function taskWorkerIds(task: CompanyTaskRecord) {
  return Array.from(new Set((task.steps || []).map(step => asString(step.workerAgentId)).filter(Boolean)));
}

export function agentStatusTone(status?: string) {
  switch (asString(status).toLowerCase()) {
    case 'completed':
      return 'bg-emerald-50 text-emerald-700 dark:bg-emerald-500/10 dark:text-emerald-200';
    case 'completed_with_errors':
    case 'partial_failed':
      return 'bg-amber-50 text-amber-700 dark:bg-amber-500/10 dark:text-amber-200';
    case 'running':
      return 'bg-blue-50 text-blue-700 dark:bg-blue-500/10 dark:text-blue-200';
    case 'failed':
      return 'bg-rose-50 text-rose-700 dark:bg-rose-500/10 dark:text-rose-200';
    case 'blocked':
      return 'bg-orange-50 text-orange-700 dark:bg-orange-500/10 dark:text-orange-200';
    default:
      return 'bg-slate-100 text-slate-600 dark:bg-slate-800 dark:text-slate-300';
  }
}
