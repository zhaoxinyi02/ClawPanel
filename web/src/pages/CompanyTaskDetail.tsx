import { useEffect, useMemo, useState } from 'react';
import { Link, useOutletContext, useParams } from 'react-router-dom';
import { Bot, ChevronLeft, Loader2, Sparkles, Users } from 'lucide-react';
import CompanyTaskWorkspace from '../components/company/CompanyTaskWorkspace';
import {
  CompanyAgentOption,
  CompanyBindingRecord,
  CompanyTaskRecord,
  CompanyTeamRecord,
  agentLabel,
  buildCompanyAgentAccessMap,
  buildCompanyAgentOptions,
  buildTaskFallbackTeam,
  normalizeCompanyTeam,
  taskWorkerIds,
} from '../components/company/types';
import { api } from '../lib/api';

export default function CompanyTaskDetail() {
  const { uiMode } = (useOutletContext() as { uiMode?: 'modern' }) || {};
  const { id = '' } = useParams();
  const [task, setTask] = useState<CompanyTaskRecord | null>(null);
  const [team, setTeam] = useState<CompanyTeamRecord | null>(null);
  const [agents, setAgents] = useState<CompanyAgentOption[]>([]);
  const [bindings, setBindings] = useState<CompanyBindingRecord[]>([]);
  const [defaultAgentId, setDefaultAgentId] = useState('main');
  const [channelConfigs, setChannelConfigs] = useState<Record<string, unknown>>({});
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    (async () => {
      try {
        const [taskRes, agentRes, channelsRes] = await Promise.all([api.getCompanyTaskDetail(id), api.getAgentsConfig(), api.getChannels()]);
        const nextTask = taskRes?.ok ? (taskRes.task as CompanyTaskRecord) : null;
        setTask(nextTask);
        if (agentRes?.ok) {
          setAgents(buildCompanyAgentOptions(agentRes?.agents?.list));
          setBindings(Array.isArray(agentRes?.agents?.bindings) ? agentRes.agents.bindings : []);
          setDefaultAgentId(String(agentRes?.agents?.default || 'main').trim() || 'main');
        }
        if (channelsRes?.ok) setChannelConfigs((channelsRes.channels || {}) as Record<string, unknown>);
        if (nextTask?.teamId) {
          const teamRes = await api.getCompanyTeamDetail(nextTask.teamId);
          if (teamRes?.ok) setTeam(teamRes.team as CompanyTeamRecord);
        }
      } finally {
        setLoading(false);
      }
    })();
  }, [id]);

  const normalizedTeam = useMemo(() => {
    if (!task) return null;
    if (team) {
      const normalized = normalizeCompanyTeam(team, agents);
      const missingWorkers = taskWorkerIds(task).filter(workerId => !normalized.workers.some(worker => worker.agentId === workerId));
      if (missingWorkers.length === 0) return normalized;
      return normalizeCompanyTeam({
        ...team,
        agents: [
          ...(team.agents || []),
          ...missingWorkers.map((workerAgentId, index) => ({ agentId: workerAgentId, roleType: 'worker', dutyLabel: `兼容执行 ${index + 1}`, enabled: true })),
        ],
      }, agents);
    }
    return buildTaskFallbackTeam(task, agents);
  }, [agents, task, team]);

  const memberIds = useMemo(() => {
    if (!normalizedTeam) return [];
    return Array.from(new Set([normalizedTeam.manager?.agentId || '', ...normalizedTeam.workers.map(worker => worker.agentId), task?.summaryAgentId || ''].filter(Boolean)));
  }, [normalizedTeam, task?.summaryAgentId]);
  const accessMap = useMemo(() => buildCompanyAgentAccessMap(memberIds, bindings, channelConfigs, defaultAgentId), [bindings, channelConfigs, defaultAgentId, memberIds]);
  const usedFallbackSummary = !!task && (task.reviewComment === 'fallback summary' || task.reviewComment === 'fallback summary from worker outputs' || task.reviewComment === 'review fallback accepted');

  return (
    <div className={uiMode === 'modern' ? 'space-y-6' : 'space-y-4'}>
      <section className="flex items-center justify-between rounded-3xl border border-slate-200/70 bg-white/90 p-6 shadow-sm dark:border-slate-700/70 dark:bg-slate-900/85">
        <div>
          <Link to="/company/tasks" className="inline-flex items-center gap-1 text-sm text-slate-500 transition hover:text-slate-700 dark:text-slate-400 dark:hover:text-slate-200"><ChevronLeft size={16} /> 返回任务中心</Link>
          <h1 className="mt-3 text-2xl font-semibold text-slate-900 dark:text-slate-100">任务详情</h1>
        </div>
        {task && <div className="rounded-full bg-slate-100 px-3 py-1 text-sm text-slate-700 dark:bg-slate-800 dark:text-slate-300">{task.status}</div>}
      </section>

      {loading ? (
        <div className="flex min-h-[280px] items-center justify-center rounded-3xl border border-slate-200/70 bg-white/90 dark:border-slate-700/70 dark:bg-slate-900/85">
          <Loader2 className="mr-2 animate-spin" size={18} /> 加载中...
        </div>
      ) : !task ? (
        <div className="rounded-3xl border border-slate-200/70 bg-white/90 p-6 text-sm text-slate-500 dark:border-slate-700/70 dark:bg-slate-900/85 dark:text-slate-400">任务不存在。</div>
      ) : (
        <>
          {usedFallbackSummary && (
            <section className="rounded-2xl border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-700 dark:border-amber-500/30 dark:bg-amber-500/10 dark:text-amber-200">
              当前结果由系统自动整理生成，建议结合下方步骤结果与执行记录一起查看，以便更完整地了解任务过程。
            </section>
          )}

          <section className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
            <div className="rounded-3xl border border-slate-200/70 bg-white/90 p-5 shadow-sm dark:border-slate-700/70 dark:bg-slate-900/85 xl:col-span-2">
              <div className="text-xs text-slate-500 dark:text-slate-400">任务标题</div>
              <div className="mt-2 text-xl font-semibold text-slate-900 dark:text-slate-100">{task.title}</div>
              <div className="mt-4 whitespace-pre-wrap text-sm leading-6 text-slate-600 dark:text-slate-300">{task.goal}</div>
            </div>
            <div className="rounded-3xl border border-amber-200/70 bg-amber-50/80 p-5 shadow-sm dark:border-amber-500/20 dark:bg-amber-500/10">
              <div className="flex items-center gap-2 text-xs text-amber-700 dark:text-amber-200"><Bot size={14} /> Manager</div>
              <div className="mt-3 text-base font-semibold text-slate-900 dark:text-slate-100">{normalizedTeam?.manager ? agentLabel(normalizedTeam.manager) : task.managerAgentId || '未配置'}</div>
              <div className="mt-2 text-sm text-slate-600 dark:text-slate-300">负责调度、拆解和最终结果汇总</div>
            </div>
            <div className="rounded-3xl border border-blue-200/70 bg-blue-50/80 p-5 shadow-sm dark:border-blue-500/20 dark:bg-blue-500/10">
              <div className="flex items-center gap-2 text-xs text-blue-700 dark:text-blue-200"><Users size={14} /> Workers</div>
              <div className="mt-3 text-base font-semibold text-slate-900 dark:text-slate-100">{taskWorkerIds(task).length} 个执行智能体</div>
              <div className="mt-2 text-sm text-slate-600 dark:text-slate-300">按任务步骤承担实际执行工作</div>
            </div>
          </section>

          <section className="grid gap-4 md:grid-cols-4">
            <div className="rounded-3xl border border-slate-200/70 bg-white/90 p-5 shadow-sm dark:border-slate-700/70 dark:bg-slate-900/85">
              <div className="text-xs text-slate-500 dark:text-slate-400">来源 / 投递</div>
              <div className="mt-2 font-medium text-slate-900 dark:text-slate-100">{task.sourceType || 'panel_manual'} / {task.deliveryType || 'notify_only'}</div>
            </div>
            <div className="rounded-3xl border border-slate-200/70 bg-white/90 p-5 shadow-sm dark:border-slate-700/70 dark:bg-slate-900/85">
              <div className="text-xs text-slate-500 dark:text-slate-400">来源通道</div>
              <div className="mt-2 font-medium text-slate-900 dark:text-slate-100">{task.sourceChannelType || task.sourceChannelId || '未指定'}</div>
            </div>
            <div className="rounded-3xl border border-slate-200/70 bg-white/90 p-5 shadow-sm dark:border-slate-700/70 dark:bg-slate-900/85">
              <div className="flex items-center gap-2 text-xs text-slate-500 dark:text-slate-400"><Sparkles size={14} /> 汇总角色</div>
              <div className="mt-2 font-medium text-slate-900 dark:text-slate-100">{task.summaryAgentId || task.managerAgentId || '未配置'}</div>
            </div>
            <div className="rounded-3xl border border-slate-200/70 bg-white/90 p-5 shadow-sm dark:border-slate-700/70 dark:bg-slate-900/85">
              <div className="text-xs text-slate-500 dark:text-slate-400">更新时间</div>
              <div className="mt-2 font-medium text-slate-900 dark:text-slate-100">{new Date(task.updatedAt).toLocaleString()}</div>
            </div>
          </section>

          <CompanyTaskWorkspace
            task={task}
            team={normalizedTeam}
            accessMap={accessMap}
            bindings={bindings}
            channelsRaw={channelConfigs}
            agentOptions={agents}
            onBindingsSaved={setBindings}
          />
        </>
      )}
    </div>
  );
}
