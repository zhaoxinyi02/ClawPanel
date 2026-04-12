import { useEffect, useMemo, useState } from 'react';
import { Link, useOutletContext } from 'react-router-dom';
import { Bot, ChevronDown, Loader2, PencilLine, Plus, Radio, Route, Users } from 'lucide-react';
import CompanyAgentInfoCard from '../components/company/CompanyAgentInfoCard';
import CompanyOrgChart from '../components/company/CompanyOrgChart';
import CompanyTeamEditorModal, { CompanyTeamEditorPayload } from '../components/company/CompanyTeamEditorModal';
import {
  CompanyAgentAccessSummary,
  CompanyAgentOption,
  CompanyBindingRecord,
  detectCompanyBindingDuplicates,
  CompanyTeamMember,
  CompanyTeamRecord,
  agentLabel,
  buildCompanyAgentAccessMap,
  buildCompanyAgentOptions,
  normalizeCompanyTeam,
  pickPreferredCompanyManager,
  retargetAgentChannelBindingAccount,
  serializeCompanyBindingRecords,
  summarizeCompanyTeamAccess,
} from '../components/company/types';
import { api } from '../lib/api';

function renderAccessHint(summary?: CompanyAgentAccessSummary) {
  if (!summary) return '尚未配置接入规则';
  if (summary.routeCount > 0) {
    return `${summary.routeCount} 条显式路由 · ${summary.channelCount} 个命中通道`;
  }
  if (summary.defaultFallback) return '当前是默认回落 Agent，未命中规则时仍会接收消息';
  return '尚未配置显式 routing / binding';
}

function memberChannelLink(summary?: CompanyAgentAccessSummary) {
  const firstChannel = summary?.channels?.[0]?.id;
  return firstChannel ? `/channels?channel=${encodeURIComponent(firstChannel)}` : '/channels';
}

export default function CompanyTeams() {
  const { uiMode } = (useOutletContext() as { uiMode?: 'modern' }) || {};
  const [teams, setTeams] = useState<CompanyTeamRecord[]>([]);
  const [agents, setAgents] = useState<CompanyAgentOption[]>([]);
  const [bindings, setBindings] = useState<CompanyBindingRecord[]>([]);
  const [draftBindings, setDraftBindings] = useState<CompanyBindingRecord[]>([]);
  const [defaultAgentId, setDefaultAgentId] = useState('main');
  const [channelConfigs, setChannelConfigs] = useState<Record<string, unknown>>({});
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [editorOpen, setEditorOpen] = useState(false);
  const [editingTeam, setEditingTeam] = useState<CompanyTeamRecord | null>(null);
  const [selectedMembers, setSelectedMembers] = useState<Record<string, CompanyTeamMember | null>>({});
  const [error, setError] = useState('');

  const load = async () => {
    const [teamRes, agentRes, channelsRes] = await Promise.all([api.getCompanyTeams(), api.getAgentsConfig(), api.getChannels()]);
    if (teamRes?.ok) setTeams(Array.isArray(teamRes.teams) ? teamRes.teams : []);
    if (agentRes?.ok) {
      setAgents(buildCompanyAgentOptions(agentRes?.agents?.list));
      const nextBindings = Array.isArray(agentRes?.agents?.bindings) ? agentRes.agents.bindings : [];
      setBindings(nextBindings);
      setDraftBindings(nextBindings);
      setDefaultAgentId(String(agentRes?.agents?.default || 'main').trim() || 'main');
    }
    if (channelsRes?.ok) setChannelConfigs((channelsRes.channels || {}) as Record<string, unknown>);
  };

  useEffect(() => {
    (async () => {
      try {
        await load();
      } catch {
        setError('团队数据加载失败');
      } finally {
        setLoading(false);
      }
    })();
  }, []);

  const normalizedTeams = useMemo(() => teams.map(team => normalizeCompanyTeam(team, agents)), [agents, teams]);
  const preferredManager = useMemo(() => pickPreferredCompanyManager(agents, normalizedTeams[0]?.managerAgentId), [agents, normalizedTeams]);
  const totalWorkers = useMemo(() => new Set(normalizedTeams.flatMap(team => team.workers.map(worker => worker.agentId))).size, [normalizedTeams]);
  const memberIds = useMemo(() => Array.from(new Set(normalizedTeams.flatMap(team => [team.manager?.agentId || '', ...team.workers.map(worker => worker.agentId)]).filter(Boolean))), [normalizedTeams]);
  const accessMap = useMemo(() => buildCompanyAgentAccessMap(memberIds, draftBindings, channelConfigs, defaultAgentId), [draftBindings, channelConfigs, defaultAgentId, memberIds]);
  const duplicateWarnings = useMemo(() => detectCompanyBindingDuplicates(draftBindings, channelConfigs), [draftBindings, channelConfigs]);
  const hasPendingBindingChanges = useMemo(() => JSON.stringify(serializeCompanyBindingRecords(draftBindings)) !== JSON.stringify(serializeCompanyBindingRecords(bindings)), [bindings, draftBindings]);

  const stageAccountChange = (agentId: string, channelId: string, accountId: string) => {
    const { nextBindings } = retargetAgentChannelBindingAccount(draftBindings, agentId, channelId, accountId);
    setDraftBindings(nextBindings);
  };

  const applyBindingChanges = async () => {
    setError('');
    const res = await api.updateBindings(serializeCompanyBindingRecords(draftBindings));
    if (!res?.ok) {
      setError(res?.error || '账号绑定应用失败');
      return;
    }
    setBindings(draftBindings);
  };

  const handleSaveTeam = async (payload: CompanyTeamEditorPayload) => {
    setSaving(true);
    setError('');
    try {
      const res = editingTeam
        ? await api.updateCompanyTeam(editingTeam.id, payload)
        : await api.createCompanyTeam(payload);
      if (!res?.ok) {
        setError(res?.error || '团队保存失败');
        return;
      }
      setEditorOpen(false);
      setEditingTeam(null);
      await load();
    } catch {
      setError('团队保存失败');
    } finally {
      setSaving(false);
    }
  };

  const renderMemberAccessCard = (member: CompanyTeamMember, roleLabel: string) => {
    const summary = accessMap[member.agentId];
    return (
      <div key={`${roleLabel}-${member.agentId}`} className="rounded-2xl border border-slate-200/70 bg-white/80 px-4 py-4 dark:border-slate-700/70 dark:bg-slate-950/40">
        <div className="flex flex-wrap items-start justify-between gap-3">
          <div>
            <div className="flex items-center gap-2">
              <div className="font-medium text-slate-900 dark:text-slate-100">{agentLabel(member)}</div>
              <span className={`rounded-full px-2 py-1 text-[11px] ${roleLabel === 'Manager' ? 'bg-amber-50 text-amber-700 dark:bg-amber-500/10 dark:text-amber-200' : 'bg-blue-50 text-blue-700 dark:bg-blue-500/10 dark:text-blue-200'}`}>{roleLabel}</span>
            </div>
            <div className="mt-1 text-xs text-slate-500 dark:text-slate-400">{renderAccessHint(summary)}</div>
          </div>
          <div className="rounded-full bg-slate-100 px-2.5 py-1 text-[11px] text-slate-600 dark:bg-slate-800 dark:text-slate-300">route {summary?.routeCount || 0}</div>
        </div>

        <div className="mt-3 flex flex-wrap gap-2 text-[11px] text-slate-500 dark:text-slate-400">
          {(summary?.channels || []).length > 0 ? summary?.channels.slice(0, 4).map(channel => (
            <span key={`${member.agentId}-${channel.id}`} className="rounded-full bg-slate-100 px-2.5 py-1 dark:bg-slate-800">
              {channel.label}
              {channel.explicitAccounts.length > 0 ? ` · ${channel.explicitAccounts.join(', ')}` : channel.defaultAccount ? ` · 默认账号 ${channel.defaultAccount}` : ''}
            </span>
          )) : <span className="rounded-full bg-slate-100 px-2.5 py-1 dark:bg-slate-800">尚无显式通道绑定</span>}
        </div>

        <div className="mt-4 flex flex-wrap gap-2">
          <Link to={`/agents?agent=${encodeURIComponent(member.agentId)}&view=routing`} className="inline-flex items-center gap-1 rounded-xl border border-slate-200 px-3 py-1.5 text-xs font-medium text-slate-700 transition hover:bg-slate-50 dark:border-slate-700 dark:text-slate-200 dark:hover:bg-slate-800">
            <Route size={13} /> 查看路由规则
          </Link>
          <Link to={memberChannelLink(summary)} className="inline-flex items-center gap-1 rounded-xl border border-slate-200 px-3 py-1.5 text-xs font-medium text-slate-700 transition hover:bg-slate-50 dark:border-slate-700 dark:text-slate-200 dark:hover:bg-slate-800">
            <Radio size={13} /> 查看通道配置
          </Link>
        </div>
      </div>
    );
  };

  return (
    <div className={uiMode === 'modern' ? 'space-y-6' : 'space-y-4'}>
      <section className="rounded-3xl border border-slate-200/70 bg-white/90 p-6 shadow-sm dark:border-slate-700/70 dark:bg-slate-900/85">
        <div className="flex flex-col gap-4 xl:flex-row xl:items-center xl:justify-between">
          <div>
            <div className="inline-flex items-center gap-2 rounded-full border border-amber-200 bg-amber-50 px-3 py-1 text-xs font-medium text-amber-700 dark:border-amber-500/20 dark:bg-amber-500/10 dark:text-amber-200">
              <Bot size={14} />
              AI 公司协作团队
            </div>
            <h1 className="mt-3 text-2xl font-semibold text-slate-900 dark:text-slate-100">团队与成员</h1>
            <p className="mt-2 max-w-3xl text-sm text-slate-500 dark:text-slate-400">在这里管理团队成员、查看协作关系，并了解每位成员当前已接入的账号与通道情况。</p>
          </div>
          <button
            onClick={() => {
              setEditingTeam(null);
              setEditorOpen(true);
            }}
            className="inline-flex items-center justify-center gap-2 rounded-2xl bg-slate-900 px-4 py-2.5 text-sm font-medium text-white transition hover:bg-slate-800 dark:bg-white dark:text-slate-900 dark:hover:bg-slate-100"
          >
            <Plus size={16} /> 新建团队
          </button>
        </div>
      </section>

      {error && <section className="rounded-2xl border border-rose-200 bg-rose-50 px-4 py-3 text-sm text-rose-700 dark:border-rose-500/30 dark:bg-rose-500/10 dark:text-rose-200">{error}</section>}
      {duplicateWarnings.length > 0 && (
        <section className="rounded-2xl border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-700 dark:border-amber-500/30 dark:bg-amber-500/10 dark:text-amber-200">
          检测到账号重复匹配：
          {duplicateWarnings.map(item => ` ${item.channelLabel}/${item.accountId} -> ${item.agentIds.join('、')}`).join('；')}
          。请调整到一账号只对应一个智能体。
        </section>
      )}

      {loading ? (
        <div className="flex min-h-[240px] items-center justify-center rounded-3xl border border-slate-200/70 bg-white/90 dark:border-slate-700/70 dark:bg-slate-900/85">
          <Loader2 className="mr-2 animate-spin" size={18} /> 加载中...
        </div>
      ) : (
        <>
          <section className="grid gap-4 md:grid-cols-2">
            <div className="rounded-3xl border border-slate-200/70 bg-white/90 p-5 shadow-sm dark:border-slate-700/70 dark:bg-slate-900/85">
              <div className="text-sm text-slate-500 dark:text-slate-400">执行团队</div>
              <div className="mt-2 text-2xl font-semibold text-slate-900 dark:text-slate-100">{normalizedTeams.length}</div>
              <p className="mt-2 text-sm text-slate-500 dark:text-slate-400">按任务类型组织成员，便于快速分配协作工作。</p>
            </div>

            <div className="rounded-3xl border border-slate-200/70 bg-white/90 p-5 shadow-sm dark:border-slate-700/70 dark:bg-slate-900/85">
              <div className="text-sm text-slate-500 dark:text-slate-400">执行成员</div>
              <div className="mt-2 text-2xl font-semibold text-slate-900 dark:text-slate-100">{totalWorkers}</div>
              <p className="mt-2 text-sm text-slate-500 dark:text-slate-400">已加入团队并可参与协作任务的智能体数量。</p>
            </div>
          </section>

          <section className="space-y-4">
            {normalizedTeams.length === 0 && (
              <div className="rounded-3xl border border-dashed border-slate-300/80 bg-slate-50/70 px-6 py-10 text-center text-sm text-slate-500 dark:border-slate-700 dark:bg-slate-900/40 dark:text-slate-400">
                还没有执行团队，先创建一个由 Manager + Workers 组成的协作编组。
              </div>
            )}

            {normalizedTeams.map(team => {
              const usesPreferredManager = preferredManager?.id ? team.manager?.agentId === preferredManager.id : false;
              const accessSummary = summarizeCompanyTeamAccess(team, accessMap);
              return (
                <section key={team.id} className="rounded-3xl border border-slate-200/70 bg-white/90 p-6 shadow-sm dark:border-slate-700/70 dark:bg-slate-900/85">
                  <div className="flex flex-col gap-5 xl:flex-row xl:items-start xl:justify-between">
                    <div className="max-w-2xl">
                      <div className="flex flex-wrap items-center gap-2">
                        <h2 className="text-xl font-semibold text-slate-900 dark:text-slate-100">{team.name}</h2>
                        <span className="rounded-full bg-slate-100 px-2.5 py-1 text-xs text-slate-600 dark:bg-slate-800 dark:text-slate-300">{team.status || 'active'}</span>
                        {!usesPreferredManager && <span className="rounded-full bg-blue-50 px-2.5 py-1 text-xs text-blue-700 dark:bg-blue-500/10 dark:text-blue-200">兼容自定义 Manager</span>}
                      </div>
                      <p className="mt-2 text-sm text-slate-500 dark:text-slate-400">{team.description || '用于承接一类稳定的执行任务。'}</p>
                      <div className="mt-4 grid gap-3 sm:grid-cols-2">
                        <div className="rounded-2xl border border-amber-200/70 bg-amber-50/70 px-4 py-4 dark:border-amber-500/20 dark:bg-amber-500/10">
                          <div className="text-xs text-amber-700 dark:text-amber-200">Manager</div>
                          <div className="mt-1 font-medium text-slate-900 dark:text-slate-100">{team.manager ? agentLabel(team.manager) : '未配置'}</div>
                          <div className="mt-2 text-xs text-slate-500 dark:text-slate-400">负责统一调度、拆解和汇总</div>
                        </div>
                        <div className="rounded-2xl border border-blue-200/70 bg-blue-50/70 px-4 py-4 dark:border-blue-500/20 dark:bg-blue-500/10">
                          <div className="text-xs text-blue-700 dark:text-blue-200">Workers</div>
                          <div className="mt-1 font-medium text-slate-900 dark:text-slate-100">{team.workers.length} 个执行智能体</div>
                          <div className="mt-2 flex flex-wrap gap-2 text-xs text-slate-500 dark:text-slate-400">
                            {team.workers.slice(0, 4).map(worker => <span key={worker.agentId} className="rounded-full bg-white/80 px-2.5 py-1 shadow-sm dark:bg-slate-950/50">{agentLabel(worker)}</span>)}
                            {team.workers.length > 4 && <span className="rounded-full bg-white/80 px-2.5 py-1 shadow-sm dark:bg-slate-950/50">+{team.workers.length - 4}</span>}
                          </div>
                        </div>
                      </div>
                    </div>

                    <div className="flex items-center gap-2">
                      <button
                        onClick={() => {
                          setEditingTeam(team);
                          setEditorOpen(true);
                        }}
                        className="inline-flex items-center gap-2 rounded-2xl border border-slate-200 px-4 py-2 text-sm font-medium text-slate-700 transition hover:bg-slate-50 dark:border-slate-700 dark:text-slate-200 dark:hover:bg-slate-800"
                      >
                        <PencilLine size={16} /> 编辑团队
                      </button>
                    </div>
                  </div>

                  <div className="mt-6">
                    <CompanyOrgChart
                      team={team}
                      title="团队汇报关系"
                      subtitle="这里展示当前团队的协作关系和职责分工。"
                      compact
                      selectedAgentId={selectedMembers[team.id]?.agentId}
                      onSelectAgent={member => setSelectedMembers(prev => ({ ...prev, [team.id]: member }))}
                    />
                  </div>

                  <div className="mt-4">
                    <CompanyAgentInfoCard
                      member={selectedMembers[team.id] || null}
                      agentOption={agents.find(agent => agent.id === selectedMembers[team.id]?.agentId) || null}
                      accessSummary={selectedMembers[team.id] ? accessMap[selectedMembers[team.id]!.agentId] : undefined}
                      channelsRaw={channelConfigs}
                      bindings={draftBindings}
                      duplicateWarnings={duplicateWarnings}
                      onStageAccountChange={stageAccountChange}
                      hasPendingChanges={hasPendingBindingChanges}
                    />
                    <div className="mt-3 flex flex-wrap gap-2">
                      <button onClick={() => void applyBindingChanges()} disabled={!hasPendingBindingChanges} className="inline-flex items-center gap-2 rounded-2xl bg-slate-900 px-4 py-2 text-sm font-medium text-white transition hover:bg-slate-800 disabled:opacity-50 dark:bg-white dark:text-slate-900 dark:hover:bg-slate-100">
                        应用账号切换
                      </button>
                      <Link to="/agents?view=routing" className="inline-flex items-center gap-1 rounded-xl border border-slate-200 px-3 py-2 text-xs font-medium text-slate-700 transition hover:bg-slate-50 dark:border-slate-700 dark:text-slate-200 dark:hover:bg-slate-800">
                        <Route size={13} /> 去路由页精细编辑
                      </Link>
                    </div>
                  </div>

                  <details className="mt-6 rounded-3xl border border-slate-200/70 bg-slate-50/70 px-5 py-4 dark:border-slate-700/70 dark:bg-slate-950/30">
                    <summary className="flex cursor-pointer list-none items-center justify-between gap-4 text-sm font-medium text-slate-900 dark:text-slate-100">
                      <span>成员接入情况</span>
                      <span className="inline-flex items-center gap-2 text-xs text-slate-500 dark:text-slate-400">
                        {accessSummary.coveredMemberCount}/{accessSummary.memberCount} 名成员已有接入覆盖 · {accessSummary.totalRouteCount} 条路由
                        <ChevronDown size={14} />
                      </span>
                    </summary>

                    <div className="mt-4 grid gap-3 md:grid-cols-3">
                      <div className="rounded-2xl bg-white/80 px-4 py-4 shadow-sm dark:bg-slate-900/70">
                        <div className="text-xs text-slate-500 dark:text-slate-400">已覆盖成员</div>
                        <div className="mt-1 text-2xl font-semibold text-slate-900 dark:text-slate-100">{accessSummary.coveredMemberCount}</div>
                      </div>
                      <div className="rounded-2xl bg-white/80 px-4 py-4 shadow-sm dark:bg-slate-900/70">
                        <div className="text-xs text-slate-500 dark:text-slate-400">Workers 接入覆盖</div>
                        <div className="mt-1 text-2xl font-semibold text-slate-900 dark:text-slate-100">{accessSummary.workerCoveredCount}/{team.workers.length}</div>
                      </div>
                      <div className="rounded-2xl bg-white/80 px-4 py-4 shadow-sm dark:bg-slate-900/70">
                        <div className="text-xs text-slate-500 dark:text-slate-400">涉及通道</div>
                        <div className="mt-2 flex flex-wrap gap-2">
                          {accessSummary.channelLabels.length > 0 ? accessSummary.channelLabels.slice(0, 4).map(label => <span key={label} className="rounded-full bg-slate-100 px-2 py-1 text-[11px] text-slate-600 dark:bg-slate-800 dark:text-slate-300">{label}</span>) : <span className="text-sm text-slate-500 dark:text-slate-400">暂无</span>}
                        </div>
                      </div>
                    </div>

                    <div className="mt-5 grid gap-3 lg:grid-cols-2 xl:grid-cols-3">
                      {team.manager && renderMemberAccessCard(team.manager, 'Manager')}
                      {team.workers.map(worker => renderMemberAccessCard(worker, 'Worker'))}
                    </div>

                    <div className="mt-5 flex flex-wrap gap-2">
                      <Link to="/agents?view=routing" className="inline-flex items-center gap-1 rounded-xl border border-slate-200 px-3 py-1.5 text-xs font-medium text-slate-700 transition hover:bg-slate-50 dark:border-slate-700 dark:text-slate-200 dark:hover:bg-slate-800">
                        <Route size={13} /> 去智能体页管理路由
                      </Link>
                      <Link to="/channels" className="inline-flex items-center gap-1 rounded-xl border border-slate-200 px-3 py-1.5 text-xs font-medium text-slate-700 transition hover:bg-slate-50 dark:border-slate-700 dark:text-slate-200 dark:hover:bg-slate-800">
                        <Radio size={13} /> 去通道页管理账号
                      </Link>
                    </div>
                  </details>
                </section>
              );
            })}
          </section>
        </>
      )}

      <CompanyTeamEditorModal
        open={editorOpen}
        saving={saving}
        agents={agents}
        team={editingTeam}
        onClose={() => {
          if (saving) return;
          setEditorOpen(false);
          setEditingTeam(null);
        }}
        onSubmit={handleSaveTeam}
      />
    </div>
  );
}
