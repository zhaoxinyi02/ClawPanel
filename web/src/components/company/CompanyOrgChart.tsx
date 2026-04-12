import { Bot, Crown, GitBranch, ShieldCheck } from 'lucide-react';
import { CompanyTaskRecord, CompanyTeamMember, NormalizedCompanyTeam, agentLabel, agentStatusTone } from './types';

type WorkerLoad = {
  total: number;
  completed: number;
  running: number;
  failed: number;
};

type Props = {
  team: NormalizedCompanyTeam | null;
  task?: CompanyTaskRecord | null;
  title?: string;
  subtitle?: string;
  compact?: boolean;
  hideIdleBadge?: boolean;
  selectedAgentId?: string;
  onSelectAgent?: (member: CompanyTeamMember) => void;
};

function buildWorkerLoad(task?: CompanyTaskRecord | null) {
  const map: Record<string, WorkerLoad> = {};
  for (const step of task?.steps || []) {
    const workerId = step.workerAgentId?.trim();
    if (!workerId) continue;
    if (!map[workerId]) {
      map[workerId] = { total: 0, completed: 0, running: 0, failed: 0 };
    }
    map[workerId].total += 1;
    if (step.status === 'completed') map[workerId].completed += 1;
    if (step.status === 'running') map[workerId].running += 1;
    if (step.status === 'failed') map[workerId].failed += 1;
  }
  return map;
}

export default function CompanyOrgChart({ team, task, title, subtitle, compact = false, hideIdleBadge = false, selectedAgentId, onSelectAgent }: Props) {
  if (!team?.manager) {
    return (
      <section className="rounded-3xl border border-dashed border-slate-300/80 bg-slate-50/70 p-5 text-sm text-slate-500 dark:border-slate-700 dark:bg-slate-900/40 dark:text-slate-400">
        尚未建立可展示的管理关系。
      </section>
    );
  }

  const workerLoad = buildWorkerLoad(task);
  const clickable = typeof onSelectAgent === 'function';

  return (
    <section className="rounded-3xl border border-slate-200/70 bg-white/90 p-5 shadow-sm dark:border-slate-700/70 dark:bg-slate-900/85">
      {(title || subtitle) && (
        <div className="mb-4 flex items-start justify-between gap-4">
          <div>
            {title && <h3 className="text-lg font-semibold text-slate-900 dark:text-slate-100">{title}</h3>}
            {subtitle && <p className="mt-1 text-sm text-slate-500 dark:text-slate-400">{subtitle}</p>}
          </div>
          <div className="rounded-full bg-amber-50 px-3 py-1 text-xs font-medium text-amber-700 dark:bg-amber-500/10 dark:text-amber-200">
            公司 -&gt; Manager -&gt; Workers
          </div>
        </div>
      )}

      <div className="flex flex-col items-center">
        <button
          type="button"
          onClick={() => team.manager && onSelectAgent?.(team.manager)}
          disabled={!clickable}
          className={`w-full max-w-md rounded-[28px] border border-amber-200/80 bg-[linear-gradient(135deg,rgba(255,251,235,0.95),rgba(255,255,255,0.96))] p-5 text-center shadow-[0_16px_48px_rgba(251,191,36,0.12)] dark:border-amber-500/20 dark:bg-[linear-gradient(135deg,rgba(120,53,15,0.28),rgba(15,23,42,0.92))] ${clickable ? 'transition hover:-translate-y-0.5 hover:shadow-[0_18px_52px_rgba(251,191,36,0.18)]' : ''} ${selectedAgentId === team.manager.agentId ? 'ring-2 ring-amber-400 ring-offset-2 dark:ring-offset-slate-900' : ''}`}
        >
          <div className="mx-auto flex h-14 w-14 items-center justify-center rounded-2xl bg-amber-100 text-amber-700 dark:bg-amber-500/15 dark:text-amber-200">
            <Crown size={26} />
          </div>
          <div className="mt-3 inline-flex items-center gap-2 rounded-full bg-white/80 px-3 py-1 text-xs font-medium text-slate-600 shadow-sm dark:bg-slate-950/70 dark:text-slate-300">
            <ShieldCheck size={13} />
            全局调度与最终汇总
          </div>
          <div className="mt-3 text-base font-semibold text-slate-900 dark:text-slate-100">{agentLabel(team.manager)}</div>
          <div className="mt-1 text-sm text-slate-500 dark:text-slate-400">{team.manager?.dutyLabel || '调度 / 汇总'}</div>
          {task?.summaryAgentId && task.summaryAgentId !== team.manager.agentId && (
            <div className="mt-3 rounded-2xl border border-blue-200 bg-blue-50/80 px-3 py-2 text-xs text-blue-700 dark:border-blue-500/20 dark:bg-blue-500/10 dark:text-blue-200">
              兼容模式：结果最终由 `{task.summaryAgentId}` 汇总
            </div>
          )}
          {clickable && <div className="mt-3 text-[11px] text-slate-500 dark:text-slate-400">点击卡片查看智能体详情与通道账号切换</div>}
        </button>

        <div className="flex min-h-10 justify-center">
          <div className="h-10 w-px bg-gradient-to-b from-amber-300/80 to-slate-300/70 dark:from-amber-400/60 dark:to-slate-700" />
        </div>

        <div className="grid w-full gap-3 sm:grid-cols-2 xl:grid-cols-3">
          {team.workers.length === 0 && (
            <div className="col-span-full rounded-2xl border border-dashed border-slate-300/80 bg-slate-50/70 px-4 py-5 text-center text-sm text-slate-500 dark:border-slate-700 dark:bg-slate-900/40 dark:text-slate-400">
              当前团队还没有配置 Worker 智能体。
            </div>
          )}

          {team.workers.map(worker => {
            const load = workerLoad[worker.agentId] || { total: 0, completed: 0, running: 0, failed: 0 };
            const hasTaskLoad = load.total > 0;
            const status = load.failed > 0 ? 'failed' : load.running > 0 ? 'running' : hasTaskLoad ? 'completed' : '';
            return (
              <button
                type="button"
                key={worker.agentId}
                onClick={() => onSelectAgent?.(worker)}
                disabled={!clickable}
                className={`${compact ? 'p-4' : 'p-5'} rounded-[26px] border text-left transition ${hasTaskLoad ? 'border-blue-200 bg-[linear-gradient(180deg,rgba(239,246,255,0.95),rgba(255,255,255,0.96))] shadow-[0_12px_36px_rgba(59,130,246,0.12)] dark:border-blue-500/20 dark:bg-[linear-gradient(180deg,rgba(30,41,59,0.96),rgba(15,23,42,0.96))]' : 'border-slate-200/80 bg-white/95 dark:border-slate-700/80 dark:bg-slate-900/92'} ${clickable ? 'hover:-translate-y-0.5 hover:shadow-[0_14px_40px_rgba(59,130,246,0.14)]' : ''} ${selectedAgentId === worker.agentId ? 'ring-2 ring-blue-400 ring-offset-2 dark:ring-offset-slate-900' : ''}`}
              >
                <div className="flex items-start justify-between gap-3">
                  <div className="flex items-center gap-3">
                    <div className="flex h-11 w-11 items-center justify-center rounded-2xl bg-slate-100 text-slate-700 dark:bg-slate-800 dark:text-slate-200">
                      <Bot size={20} />
                    </div>
                    <div>
                      <div className="text-sm font-semibold text-slate-900 dark:text-slate-100">{agentLabel(worker)}</div>
                      <div className="mt-1 text-xs text-slate-500 dark:text-slate-400">{worker.dutyLabel || '执行'}</div>
                    </div>
                  </div>
                    {(hasTaskLoad || !hideIdleBadge) && (
                      <div className={`rounded-full px-2.5 py-1 text-[11px] font-medium ${agentStatusTone(status)}`}>
                        {hasTaskLoad ? `${load.total} 步` : '待命'}
                      </div>
                    )}
                </div>

                <div className="mt-4 flex flex-wrap gap-2 text-[11px] text-slate-500 dark:text-slate-400">
                  <span className="inline-flex items-center gap-1 rounded-full bg-slate-100 px-2.5 py-1 dark:bg-slate-800">
                    <GitBranch size={12} />
                    Worker
                  </span>
                  {hasTaskLoad && (
                    <>
                      <span className="rounded-full bg-emerald-50 px-2.5 py-1 text-emerald-700 dark:bg-emerald-500/10 dark:text-emerald-200">完成 {load.completed}</span>
                      {load.running > 0 && <span className="rounded-full bg-blue-50 px-2.5 py-1 text-blue-700 dark:bg-blue-500/10 dark:text-blue-200">进行中 {load.running}</span>}
                      {load.failed > 0 && <span className="rounded-full bg-rose-50 px-2.5 py-1 text-rose-700 dark:bg-rose-500/10 dark:text-rose-200">失败 {load.failed}</span>}
                    </>
                  )}
                </div>
                {clickable && <div className="mt-3 text-[11px] text-slate-400 dark:text-slate-500">点击查看详情与切换该通道账号</div>}
              </button>
            );
          })}
        </div>
      </div>
    </section>
  );
}
