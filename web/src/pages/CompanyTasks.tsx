import { useEffect, useMemo, useState } from 'react';
import { Link, useOutletContext } from 'react-router-dom';
import { BriefcaseBusiness, Loader2, Plus, Radio, Route, X } from 'lucide-react';
import CompanyOrgChart from '../components/company/CompanyOrgChart';
import {
  CompanyAgentAccessSummary,
  CompanyAgentOption,
  CompanyBindingRecord,
  CompanyTaskRecord,
  CompanyTeamRecord,
  agentLabel,
  buildCompanyAgentAccessMap,
  buildCompanyAgentOptions,
  normalizeCompanyTeam,
  pickPreferredCompanyManager,
  summarizeCompanyTeamAccess,
  taskWorkerIds,
} from '../components/company/types';
import { api } from '../lib/api';

const INITIAL_FORM = {
  teamId: 'default',
  title: '',
  goal: '',
};

function coverageText(summary?: CompanyAgentAccessSummary) {
  if (!summary) return '暂无接入配置';
  if (summary.routeCount > 0) return `${summary.routeCount} 条路由 · ${summary.channelCount} 个通道上下文`;
  if (summary.defaultFallback) return '默认回落 Agent';
  return '暂无显式路由';
}

export default function CompanyTasks() {
  const { uiMode } = (useOutletContext() as { uiMode?: 'modern' }) || {};
  const [tasks, setTasks] = useState<CompanyTaskRecord[]>([]);
  const [teams, setTeams] = useState<CompanyTeamRecord[]>([]);
  const [agents, setAgents] = useState<CompanyAgentOption[]>([]);
  const [bindings, setBindings] = useState<CompanyBindingRecord[]>([]);
  const [defaultAgentId, setDefaultAgentId] = useState('main');
  const [channelConfigs, setChannelConfigs] = useState<Record<string, unknown>>({});
  const [loading, setLoading] = useState(true);
  const [creating, setCreating] = useState(false);
  const [modalOpen, setModalOpen] = useState(false);
  const [error, setError] = useState('');
  const [notice, setNotice] = useState('');
  const [form, setForm] = useState(INITIAL_FORM);

  const load = async () => {
    const [taskRes, teamRes, agentRes, channelsRes] = await Promise.all([
      api.getCompanyTasks(),
      api.getCompanyTeams(),
      api.getAgentsConfig(),
      api.getChannels(),
    ]);
    if (taskRes?.ok) setTasks(Array.isArray(taskRes.tasks) ? taskRes.tasks : []);
    if (teamRes?.ok) {
      const nextTeams = Array.isArray(teamRes.teams) ? teamRes.teams : [];
      setTeams(nextTeams);
      setForm(current => ({ ...current, teamId: current.teamId || nextTeams[0]?.id || 'default' }));
    }
    if (agentRes?.ok) {
      setAgents(buildCompanyAgentOptions(agentRes?.agents?.list));
      setBindings(Array.isArray(agentRes?.agents?.bindings) ? agentRes.agents.bindings : []);
      setDefaultAgentId(String(agentRes?.agents?.default || 'main').trim() || 'main');
    }
    if (channelsRes?.ok) setChannelConfigs((channelsRes.channels || {}) as Record<string, unknown>);
  };

  useEffect(() => {
    (async () => {
      try {
        await load();
      } finally {
        setLoading(false);
      }
    })();
  }, []);

  useEffect(() => {
    const timer = window.setInterval(() => {
      void load().catch(() => {});
    }, 5000);
    return () => window.clearInterval(timer);
  }, []);

  const normalizedTeams = useMemo(() => teams.map(team => normalizeCompanyTeam(team, agents)), [agents, teams]);
  const currentTeam = useMemo(() => normalizedTeams.find(team => team.id === form.teamId) || normalizedTeams[0] || null, [form.teamId, normalizedTeams]);
  const preferredManager = useMemo(() => pickPreferredCompanyManager(agents, currentTeam?.managerAgentId), [agents, currentTeam?.managerAgentId]);
  const teamMap = useMemo(() => Object.fromEntries(normalizedTeams.map(team => [team.id, team])), [normalizedTeams]);
  const memberIds = useMemo(() => Array.from(new Set(normalizedTeams.flatMap(team => [team.manager?.agentId || '', ...team.workers.map(worker => worker.agentId)]).filter(Boolean))), [normalizedTeams]);
  const accessMap = useMemo(() => buildCompanyAgentAccessMap(memberIds, bindings, channelConfigs, defaultAgentId), [bindings, channelConfigs, defaultAgentId, memberIds]);
  const currentTeamAccess = useMemo(() => summarizeCompanyTeamAccess(currentTeam, accessMap), [accessMap, currentTeam]);

  const closeModal = () => {
    if (creating) return;
    setModalOpen(false);
    setError('');
    setForm(current => ({ ...INITIAL_FORM, teamId: current.teamId || normalizedTeams[0]?.id || 'default' }));
  };

  const handleCreate = async () => {
    if (!form.goal.trim() || creating || !currentTeam) return;
    setCreating(true);
    setError('');
    setNotice('');
    try {
      const res = await api.createCompanyTask({
        teamId: currentTeam.id,
        title: form.title.trim(),
        goal: form.goal.trim(),
        managerAgentId: currentTeam.manager?.agentId || preferredManager?.id || 'main',
        workerAgentIds: currentTeam.workers.map(worker => worker.agentId),
        summaryAgentId: currentTeam.manager?.agentId || preferredManager?.id || 'main',
        sourceType: 'panel_manual',
        deliveryType: 'notify_only',
      });
      if (!res?.ok || !res?.task?.id) {
        setError(res?.error || '创建任务失败');
        return;
      }
      await load();
      closeModal();
      setNotice(res?.message || '任务已创建，正在后台执行。后续进度会在任务中心自动刷新。');
    } finally {
      setCreating(false);
    }
  };

  return (
    <div className={uiMode === 'modern' ? 'space-y-6' : 'space-y-4'}>
      <section className="rounded-3xl border border-slate-200/70 bg-white/90 p-6 shadow-sm dark:border-slate-700/70 dark:bg-slate-900/85">
        <div className="flex flex-col gap-4 xl:flex-row xl:items-center xl:justify-between">
          <div>
            <div className="inline-flex items-center gap-2 rounded-full border border-blue-200 bg-blue-50 px-3 py-1 text-xs font-medium text-blue-700 dark:border-blue-500/30 dark:bg-blue-500/10 dark:text-blue-200">
              <BriefcaseBusiness size={14} />
              AI 公司协作任务
            </div>
            <h1 className="mt-3 text-2xl font-semibold text-slate-900 dark:text-slate-100">任务中心</h1>
            <p className="mt-2 max-w-3xl text-sm text-slate-500 dark:text-slate-400">在这里创建、查看和跟踪协作任务。你可以为团队分配目标，并持续关注执行进度与结果。</p>
          </div>
          <button onClick={() => setModalOpen(true)} className="inline-flex items-center justify-center gap-2 rounded-2xl bg-slate-900 px-4 py-2.5 text-sm font-medium text-white transition hover:bg-slate-800 dark:bg-white dark:text-slate-900 dark:hover:bg-slate-100">
            <Plus size={16} /> 创建任务
          </button>
        </div>
      </section>

      {notice && <section className="rounded-2xl border border-emerald-200 bg-emerald-50 px-4 py-3 text-sm text-emerald-700 dark:border-emerald-500/30 dark:bg-emerald-500/10 dark:text-emerald-200">{notice}</section>}

      <section className="grid gap-6 xl:grid-cols-[minmax(0,1fr)_320px]">
        <div className="rounded-3xl border border-slate-200/70 bg-white/90 p-6 shadow-sm dark:border-slate-700/70 dark:bg-slate-900/85">
          <div className="mb-4 flex items-center justify-between">
            <div className="text-sm text-slate-500 dark:text-slate-400">共 {tasks.length} 条协作任务</div>
            <Link to="/company/teams" className="text-sm font-medium text-blue-600 dark:text-blue-300">查看执行团队</Link>
          </div>
          {loading ? (
            <div className="flex min-h-[240px] items-center justify-center"><Loader2 className="mr-2 animate-spin" size={18} />加载中...</div>
          ) : (
            <div className="space-y-3">
              {tasks.length === 0 && <div className="text-sm text-slate-500 dark:text-slate-400">暂无协作任务。</div>}
              {tasks.map(task => {
                const relatedTeam = teamMap[task.teamId || ''];
                const members = [relatedTeam?.manager?.agentId || task.managerAgentId || '', ...(relatedTeam?.workers || []).map(worker => worker.agentId)].filter(Boolean);
                const coveredMembers = members.filter(agentId => accessMap[agentId]?.hasCoverage).length;
                return (
                  <Link key={task.id} to={`/company/tasks/${task.id}`} className="block rounded-3xl border border-slate-200/70 px-5 py-5 transition hover:bg-slate-50 dark:border-slate-700/70 dark:hover:bg-slate-800/70">
                    <div className="flex items-start justify-between gap-4">
                      <div className="min-w-0">
                        <div className="font-medium text-slate-900 dark:text-slate-100">{task.title}</div>
                        <div className="mt-1 text-xs text-slate-500 dark:text-slate-400">{task.sourceType || 'panel_manual'} / {task.deliveryType || 'notify_only'} · {new Date(task.updatedAt).toLocaleString()}</div>
                      </div>
                      <div className="rounded-full bg-slate-100 px-2.5 py-1 text-xs text-slate-600 dark:bg-slate-800 dark:text-slate-300">{task.status}</div>
                    </div>
                    <div className="mt-4 grid gap-3 md:grid-cols-4">
                      <div className="rounded-2xl bg-amber-50/80 px-4 py-3 dark:bg-amber-500/10">
                        <div className="text-[11px] text-amber-700 dark:text-amber-200">Manager</div>
                        <div className="mt-1 text-sm font-medium text-slate-900 dark:text-slate-100">{task.managerAgentId || relatedTeam?.manager?.agentId || preferredManager?.id || 'main'}</div>
                      </div>
                      <div className="rounded-2xl bg-blue-50/80 px-4 py-3 dark:bg-blue-500/10">
                        <div className="text-[11px] text-blue-700 dark:text-blue-200">Workers</div>
                        <div className="mt-1 text-sm font-medium text-slate-900 dark:text-slate-100">{taskWorkerIds(task).length || relatedTeam?.workers.length || 0} 个执行智能体</div>
                      </div>
                      <div className="rounded-2xl bg-emerald-50/80 px-4 py-3 dark:bg-emerald-500/10">
                        <div className="text-[11px] text-emerald-700 dark:text-emerald-200">最终汇总</div>
                        <div className="mt-1 text-sm font-medium text-slate-900 dark:text-slate-100">{task.summaryAgentId || task.managerAgentId || preferredManager?.id || 'main'}</div>
                      </div>
                      <div className="rounded-2xl bg-slate-100/80 px-4 py-3 dark:bg-slate-800/70">
                        <div className="text-[11px] text-slate-500 dark:text-slate-400">接入覆盖</div>
                        <div className="mt-1 text-sm font-medium text-slate-900 dark:text-slate-100">{coveredMembers}/{members.length || 1} 名成员</div>
                      </div>
                    </div>
                    <div className="mt-4 line-clamp-2 text-sm text-slate-600 dark:text-slate-300">{task.goal}</div>
                  </Link>
                );
              })}
            </div>
          )}
        </div>

        <div className="space-y-6">
          <section className="rounded-3xl border border-slate-200/70 bg-white/90 p-6 shadow-sm dark:border-slate-700/70 dark:bg-slate-900/85">
            <h2 className="text-lg font-semibold text-slate-900 dark:text-slate-100">创建规则</h2>
            <div className="mt-4 space-y-3 text-sm leading-6 text-slate-600 dark:text-slate-300">
              <p>1. 任务会默认交给当前团队的主管统筹处理。</p>
              <p>2. 执行成员会根据任务步骤分别完成各自的工作。</p>
              <p>3. 如果团队成员尚未配置账号或路由，可先到对应设置页补充后再执行任务。</p>
            </div>
          </section>

          <section className="rounded-3xl border border-slate-200/70 bg-white/90 p-6 shadow-sm dark:border-slate-700/70 dark:bg-slate-900/85">
            <h2 className="text-lg font-semibold text-slate-900 dark:text-slate-100">当前团队接入覆盖</h2>
            <p className="mt-2 text-sm text-slate-500 dark:text-slate-400">这里只展示当前团队是否已经具备可执行协作任务的接入条件，便于你在派发任务前快速确认账号与路由是否就绪。</p>
            <div className="mt-4 grid gap-3">
              <div className="rounded-2xl bg-slate-50 px-4 py-3 dark:bg-slate-800/60">
                <div className="text-xs text-slate-500 dark:text-slate-400">成员接入覆盖</div>
                <div className="mt-1 text-2xl font-semibold text-slate-900 dark:text-slate-100">{currentTeamAccess.coveredMemberCount}/{currentTeamAccess.memberCount}</div>
              </div>
              <div className="rounded-2xl bg-slate-50 px-4 py-3 dark:bg-slate-800/60">
                <div className="text-xs text-slate-500 dark:text-slate-400">团队显式路由</div>
                <div className="mt-1 text-2xl font-semibold text-slate-900 dark:text-slate-100">{currentTeamAccess.totalRouteCount}</div>
              </div>
              <div className="rounded-2xl bg-slate-50 px-4 py-3 dark:bg-slate-800/60">
                <div className="text-xs text-slate-500 dark:text-slate-400">涉及通道</div>
                <div className="mt-2 flex flex-wrap gap-2">
                  {currentTeamAccess.channelLabels.length > 0 ? currentTeamAccess.channelLabels.slice(0, 4).map(label => <span key={label} className="rounded-full bg-white px-2 py-1 text-[11px] text-slate-600 dark:bg-slate-900 dark:text-slate-300">{label}</span>) : <span className="text-sm text-slate-500 dark:text-slate-400">暂无</span>}
                </div>
              </div>
            </div>
            <div className="mt-5 flex flex-wrap gap-2">
              <Link to="/agents?view=routing" className="inline-flex items-center gap-1 rounded-xl border border-slate-200 px-3 py-1.5 text-xs font-medium text-slate-700 transition hover:bg-slate-50 dark:border-slate-700 dark:text-slate-200 dark:hover:bg-slate-800"><Route size={13} /> 查看路由规则</Link>
              <Link to="/channels" className="inline-flex items-center gap-1 rounded-xl border border-slate-200 px-3 py-1.5 text-xs font-medium text-slate-700 transition hover:bg-slate-50 dark:border-slate-700 dark:text-slate-200 dark:hover:bg-slate-800"><Radio size={13} /> 查看通道配置</Link>
            </div>
          </section>
        </div>
      </section>

      {modalOpen && (
        <div className="fixed inset-0 z-[90] overflow-y-auto bg-slate-950/55 px-3 py-4 backdrop-blur-sm sm:px-4 sm:py-6 lg:px-6">
          <div className="flex min-h-full items-start justify-center">
            <div className="flex w-full max-w-6xl max-h-[calc(100vh-2rem)] flex-col overflow-hidden rounded-3xl border border-slate-200/70 bg-white shadow-2xl dark:border-slate-700/70 dark:bg-slate-900 sm:max-h-[calc(100vh-3rem)]">
              <div className="flex items-start justify-between gap-4 border-b border-slate-200/70 px-4 py-4 dark:border-slate-700/70 sm:px-6">
              <div>
                <h2 className="text-xl font-semibold text-slate-900 dark:text-slate-100">创建协作任务</h2>
                <p className="mt-1 text-sm text-slate-500 dark:text-slate-400">填写任务目标后即可发起协作，系统会自动安排主管和成员协同处理。</p>
              </div>
              <button onClick={closeModal} className="inline-flex h-10 w-10 items-center justify-center rounded-2xl border border-slate-200 text-slate-500 transition hover:bg-slate-50 dark:border-slate-700 dark:text-slate-300 dark:hover:bg-slate-800"><X size={16} /></button>
              </div>

              <div className="flex-1 overflow-y-auto px-4 py-4 sm:px-6 sm:py-5">
                <div className="grid gap-6 xl:grid-cols-[minmax(0,1.05fr)_minmax(320px,420px)] xl:items-start">
                  <div className="space-y-4 min-w-0">
                <div>
                  <div className="mb-1 text-xs text-slate-500 dark:text-slate-400">执行团队</div>
                  <select value={form.teamId} onChange={event => setForm(current => ({ ...current, teamId: event.target.value }))} className="w-full rounded-2xl border border-slate-200 bg-white px-3 py-2 text-sm dark:border-slate-700 dark:bg-slate-900">
                    {normalizedTeams.map(team => <option key={team.id} value={team.id}>{team.name}</option>)}
                  </select>
                </div>

                <div className="grid gap-4 md:grid-cols-2">
                  <div className="rounded-2xl border border-amber-200/70 bg-amber-50/80 px-4 py-4 dark:border-amber-500/20 dark:bg-amber-500/10">
                    <div className="text-xs text-amber-700 dark:text-amber-200">调度 Manager</div>
                    <div className="mt-1 font-medium text-slate-900 dark:text-slate-100">{currentTeam?.manager ? agentLabel(currentTeam.manager) : '未配置'}</div>
                    <div className="mt-2 text-xs text-slate-500 dark:text-slate-400">创建后由它负责拆解任务与汇总结果</div>
                  </div>
                  <div className="rounded-2xl border border-blue-200/70 bg-blue-50/80 px-4 py-4 dark:border-blue-500/20 dark:bg-blue-500/10">
                    <div className="text-xs text-blue-700 dark:text-blue-200">执行 Workers</div>
                    <div className="mt-1 font-medium text-slate-900 dark:text-slate-100">{currentTeam?.workers.length || 0} 个</div>
                    <div className="mt-2 text-xs text-slate-500 dark:text-slate-400">当前团队中的 Worker 会成为默认执行者</div>
                  </div>
                </div>

                <div>
                  <div className="mb-1 text-xs text-slate-500 dark:text-slate-400">任务标题</div>
                  <input value={form.title} onChange={event => setForm(current => ({ ...current, title: event.target.value }))} className="w-full rounded-2xl border border-slate-200 bg-white px-3 py-2 text-sm dark:border-slate-700 dark:bg-slate-900" placeholder="例如：首页交付改版" />
                </div>
                <div>
                  <div className="mb-1 text-xs text-slate-500 dark:text-slate-400">任务目标</div>
                  <textarea value={form.goal} onChange={event => setForm(current => ({ ...current, goal: event.target.value }))} rows={8} className="min-h-[180px] w-full rounded-2xl border border-slate-200 bg-white px-3 py-2 text-sm dark:border-slate-700 dark:bg-slate-900 xl:min-h-[220px]" placeholder="描述任务目标、上下文、约束和期望交付物" />
                </div>
                {error && <div className="rounded-2xl border border-rose-200 bg-rose-50 px-3 py-2 text-sm text-rose-700 dark:border-rose-500/30 dark:bg-rose-500/10 dark:text-rose-200">{error}</div>}
                  </div>

                  <div className="space-y-4 min-w-0 xl:sticky xl:top-0">
                    <CompanyOrgChart
                      team={currentTeam}
                      title="任务组织预览"
                      subtitle="提交后，Manager 会在顶层统一调度，Worker 负责执行拆解后的步骤。"
                      compact
                    />

                    <section className="rounded-3xl border border-slate-200/70 bg-white/90 p-5 shadow-sm dark:border-slate-700/70 dark:bg-slate-900/85">
                      <h3 className="text-base font-semibold text-slate-900 dark:text-slate-100">接入覆盖预览</h3>
                      <div className="mt-4 grid gap-3 sm:grid-cols-2 xl:grid-cols-1 2xl:grid-cols-2">
                        <div className="rounded-2xl bg-slate-50 px-4 py-3 dark:bg-slate-800/60">
                          <div className="text-xs text-slate-500 dark:text-slate-400">成员接入覆盖</div>
                          <div className="mt-1 text-xl font-semibold text-slate-900 dark:text-slate-100">{currentTeamAccess.coveredMemberCount}/{currentTeamAccess.memberCount}</div>
                        </div>
                        <div className="rounded-2xl bg-slate-50 px-4 py-3 dark:bg-slate-800/60">
                          <div className="text-xs text-slate-500 dark:text-slate-400">显式路由</div>
                          <div className="mt-1 text-xl font-semibold text-slate-900 dark:text-slate-100">{currentTeamAccess.totalRouteCount}</div>
                        </div>
                      </div>
                      <div className="mt-4 max-h-60 space-y-2 overflow-y-auto pr-1">
                        {[currentTeam?.manager, ...(currentTeam?.workers || [])].filter(Boolean).map(member => {
                          const summary = member ? accessMap[member.agentId] : undefined;
                          return (
                            <div key={member?.agentId} className="rounded-2xl border border-slate-200/70 px-4 py-3 text-sm dark:border-slate-700/70">
                              <div className="font-medium text-slate-900 dark:text-slate-100">{member ? agentLabel(member) : ''}</div>
                              <div className="mt-1 text-xs text-slate-500 dark:text-slate-400">{coverageText(summary)}</div>
                            </div>
                          );
                        })}
                      </div>
                      <div className="mt-4 flex flex-wrap gap-2">
                        <Link to="/agents?view=routing" className="inline-flex items-center gap-1 rounded-xl border border-slate-200 px-3 py-1.5 text-xs font-medium text-slate-700 transition hover:bg-slate-50 dark:border-slate-700 dark:text-slate-200 dark:hover:bg-slate-800"><Route size={13} /> 管理路由</Link>
                        <Link to="/channels" className="inline-flex items-center gap-1 rounded-xl border border-slate-200 px-3 py-1.5 text-xs font-medium text-slate-700 transition hover:bg-slate-50 dark:border-slate-700 dark:text-slate-200 dark:hover:bg-slate-800"><Radio size={13} /> 管理通道</Link>
                      </div>
                    </section>
                  </div>
                </div>
              </div>

              <div className="flex items-center justify-end gap-3 border-t border-slate-200/70 px-4 py-4 dark:border-slate-700/70 sm:px-6">
                <button onClick={closeModal} className="rounded-2xl border border-slate-200 px-4 py-2 text-sm font-medium text-slate-700 transition hover:bg-slate-50 dark:border-slate-700 dark:text-slate-200 dark:hover:bg-slate-800">取消</button>
                <button onClick={() => void handleCreate()} disabled={creating || !form.goal.trim() || !currentTeam} className="inline-flex items-center gap-2 rounded-2xl bg-slate-900 px-4 py-2 text-sm font-medium text-white transition hover:bg-slate-800 disabled:opacity-50 dark:bg-white dark:text-slate-900 dark:hover:bg-slate-100">
                  {creating ? <Loader2 className="animate-spin" size={16} /> : <Plus size={16} />}
                  创建任务
                </button>
              </div>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
