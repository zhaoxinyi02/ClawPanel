import { useEffect, useMemo, useState } from 'react';
import { Link, useOutletContext } from 'react-router-dom';
import { BriefcaseBusiness, CheckCircle2, Loader2, Sparkles, Users } from 'lucide-react';
import {
  CompanyAgentOption,
  CompanyOverviewData,
  CompanyTeamRecord,
  buildCompanyAgentOptions,
  normalizeCompanyTeam,
  taskWorkerIds,
} from '../components/company/types';
import { api } from '../lib/api';

export default function CompanyOverview() {
  const { uiMode } = (useOutletContext() as { uiMode?: 'modern' }) || {};
  const [overview, setOverview] = useState<CompanyOverviewData | null>(null);
  const [teams, setTeams] = useState<CompanyTeamRecord[]>([]);
  const [agents, setAgents] = useState<CompanyAgentOption[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    (async () => {
      try {
        const [overviewRes, teamRes, agentRes] = await Promise.all([
          api.getCompanyOverview(),
          api.getCompanyTeams(),
          api.getAgentsConfig(),
        ]);
        if (overviewRes?.ok) setOverview(overviewRes.overview || null);
        if (teamRes?.ok) setTeams(Array.isArray(teamRes.teams) ? teamRes.teams : []);
        if (agentRes?.ok) setAgents(buildCompanyAgentOptions(agentRes?.agents?.list));
      } finally {
        setLoading(false);
      }
    })();
  }, []);

  const normalizedTeams = useMemo(() => teams.map(team => normalizeCompanyTeam(team, agents)), [agents, teams]);
  const teamMap = useMemo(() => Object.fromEntries(normalizedTeams.map(team => [team.id, team])), [normalizedTeams]);
  const workerCount = useMemo(() => new Set(normalizedTeams.flatMap(team => team.workers.map(worker => worker.agentId))).size, [normalizedTeams]);
  const shellClass = uiMode === 'modern' ? 'space-y-6' : 'space-y-4';

  return (
    <div className={shellClass}>
      <section className="flex flex-col gap-4 rounded-3xl border border-slate-200/70 bg-white/90 p-6 shadow-sm dark:border-slate-700/70 dark:bg-slate-900/85 xl:flex-row xl:items-center xl:justify-between">
        <div>
          <div className="inline-flex items-center gap-2 rounded-full border border-blue-200 bg-blue-50 px-3 py-1 text-xs font-medium text-blue-700 dark:border-blue-500/30 dark:bg-blue-500/10 dark:text-blue-200">
            <BriefcaseBusiness size={14} />
            AI 公司
          </div>
          <h1 className="mt-3 text-2xl font-semibold text-slate-900 dark:text-slate-100">AI 公司总览</h1>
          <p className="mt-2 max-w-3xl text-sm text-slate-500 dark:text-slate-400">在这里查看 AI 公司的协作结构、活跃任务和成员接入情况。主管负责统筹协作，执行成员按各自分工完成工作。</p>
        </div>
        <div className="flex gap-3">
          <Link to="/company/tasks" className="inline-flex items-center justify-center rounded-2xl bg-slate-900 px-4 py-2 text-sm font-medium text-white transition hover:bg-slate-800 dark:bg-white dark:text-slate-900 dark:hover:bg-slate-100">进入任务中心</Link>
          <Link to="/company/teams" className="inline-flex items-center justify-center rounded-2xl border border-slate-200 px-4 py-2 text-sm font-medium text-slate-700 transition hover:bg-slate-50 dark:border-slate-700 dark:text-slate-200 dark:hover:bg-slate-800">查看执行团队</Link>
        </div>
      </section>

      {loading ? (
        <div className="flex min-h-[240px] items-center justify-center rounded-3xl border border-slate-200/70 bg-white/90 dark:border-slate-700/70 dark:bg-slate-900/85">
          <Loader2 className="mr-2 animate-spin" size={18} /> 加载中...
        </div>
      ) : (
        <>
          <section className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
            {[
              { label: '执行团队', value: String(overview?.teamCount || normalizedTeams.length || 0), icon: Users },
              { label: '活跃 Workers', value: String(workerCount), icon: Sparkles },
              { label: '执行中任务', value: String(overview?.runningCount || 0), icon: BriefcaseBusiness },
              { label: '已完成任务', value: String(overview?.completedCount || 0), icon: CheckCircle2 },
            ].map(item => (
              <div key={item.label} className="rounded-3xl border border-slate-200/70 bg-white/90 p-5 shadow-sm dark:border-slate-700/70 dark:bg-slate-900/85">
                <div className="flex items-center justify-between gap-3">
                  <div className="text-sm text-slate-500 dark:text-slate-400">{item.label}</div>
                  <item.icon size={18} className="text-slate-400 dark:text-slate-500" />
                </div>
                <div className="mt-4 text-lg font-semibold text-slate-900 dark:text-slate-100">{item.value}</div>
              </div>
            ))}
          </section>

          <section className="rounded-3xl border border-slate-200/70 bg-white/90 p-6 shadow-sm dark:border-slate-700/70 dark:bg-slate-900/85">
            <div className="mb-4 flex items-center justify-between">
              <h2 className="text-lg font-semibold text-slate-900 dark:text-slate-100">最近任务</h2>
              <Link to="/company/tasks" className="text-sm font-medium text-blue-600 dark:text-blue-300">查看全部</Link>
            </div>
            <div className="space-y-3">
              {(overview?.recentTasks || []).length === 0 && <div className="text-sm text-slate-500 dark:text-slate-400">暂无协作任务。</div>}
              {(overview?.recentTasks || []).map(task => (
                <Link key={task.id} to={`/company/tasks/${task.id}`} className="flex items-center justify-between gap-4 rounded-2xl border border-slate-200/70 px-4 py-4 transition hover:bg-slate-50 dark:border-slate-700/70 dark:hover:bg-slate-800/70">
                  <div className="min-w-0">
                    <div className="font-medium text-slate-900 dark:text-slate-100">{task.title}</div>
                    <div className="mt-1 text-xs text-slate-500 dark:text-slate-400">{task.managerAgentId || teamMap[task.teamId || '']?.manager?.agentId || 'manager'} 调度 · {taskWorkerIds(task).length || teamMap[task.teamId || '']?.workers.length || 0} 个 Worker · {new Date(task.updatedAt).toLocaleString()}</div>
                    <div className="mt-2 line-clamp-2 text-sm text-slate-600 dark:text-slate-300">{task.goal}</div>
                  </div>
                  <div className="rounded-full bg-slate-100 px-2.5 py-1 text-xs text-slate-600 dark:bg-slate-800 dark:text-slate-300">{task.status}</div>
                </Link>
              ))}
            </div>
          </section>
        </>
      )}
    </div>
  );
}
