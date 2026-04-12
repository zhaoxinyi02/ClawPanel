import { useEffect, useMemo, useState } from 'react';
import { Loader2, Plus, Users, X } from 'lucide-react';
import CompanyOrgChart from './CompanyOrgChart';
import {
  CompanyAgentOption,
  CompanyTeamRecord,
  NormalizedCompanyTeam,
  agentLabel,
  normalizeCompanyTeam,
  pickPreferredCompanyManager,
} from './types';

export type CompanyTeamEditorPayload = {
  name: string;
  description?: string;
  managerAgentId: string;
  workerAgentIds: string[];
};

type Props = {
  open: boolean;
  saving: boolean;
  agents: CompanyAgentOption[];
  team: CompanyTeamRecord | null;
  onClose: () => void;
  onSubmit: (payload: CompanyTeamEditorPayload) => Promise<void> | void;
};

const EMPTY_FORM: CompanyTeamEditorPayload = {
  name: '',
  description: '',
  managerAgentId: '',
  workerAgentIds: [],
};

function buildInitialForm(team: CompanyTeamRecord | null, agents: CompanyAgentOption[]): CompanyTeamEditorPayload {
  const preferredManager = pickPreferredCompanyManager(agents, team?.managerAgentId);
  const normalizedTeam = team ? normalizeCompanyTeam(team, agents) : null;
  return {
    name: team?.name || '',
    description: team?.description || '',
    managerAgentId: normalizedTeam?.manager?.agentId || preferredManager?.id || agents[0]?.id || 'main',
    workerAgentIds: normalizedTeam?.workers.map(worker => worker.agentId) || [],
  };
}

export default function CompanyTeamEditorModal({ open, saving, agents, team, onClose, onSubmit }: Props) {
  const [form, setForm] = useState<CompanyTeamEditorPayload>(EMPTY_FORM);
  const [error, setError] = useState('');

  useEffect(() => {
    if (!open) return;
    setForm(buildInitialForm(team, agents));
    setError('');
  }, [agents, open, team]);

  const preferredManager = useMemo(() => pickPreferredCompanyManager(agents, form.managerAgentId), [agents, form.managerAgentId]);

  const previewTeam = useMemo<NormalizedCompanyTeam>(() => normalizeCompanyTeam({
    id: team?.id || 'draft',
    name: form.name || '未命名团队',
    description: form.description,
    managerAgentId: form.managerAgentId,
    defaultSummaryAgentId: form.managerAgentId,
    status: team?.status || 'active',
    agents: [
      { agentId: form.managerAgentId, roleType: 'manager', dutyLabel: '调度 / 汇总', enabled: true },
      ...form.workerAgentIds.map((agentId, index) => ({ agentId, roleType: 'worker', dutyLabel: `执行 ${index + 1}`, sortOrder: index, enabled: true })),
    ],
  }, agents), [agents, form.description, form.managerAgentId, form.name, form.workerAgentIds, team?.id, team?.status]);

  if (!open) return null;

  const availableWorkers = agents.filter(agent => agent.id !== form.managerAgentId);

  const handleToggleWorker = (agentId: string) => {
    setForm(current => current.workerAgentIds.includes(agentId)
      ? { ...current, workerAgentIds: current.workerAgentIds.filter(item => item !== agentId) }
      : { ...current, workerAgentIds: [...current.workerAgentIds, agentId] });
  };

  const handleSubmit = async () => {
    if (!form.name.trim()) {
      setError('请先填写团队名称');
      return;
    }
    if (!form.managerAgentId.trim()) {
      setError('请先确认团队 Manager');
      return;
    }
    if (form.workerAgentIds.length === 0) {
      setError('至少选择 1 个 Worker 智能体');
      return;
    }
    setError('');
    await onSubmit({
      name: form.name.trim(),
      description: form.description?.trim(),
      managerAgentId: form.managerAgentId.trim(),
      workerAgentIds: form.workerAgentIds,
    });
  };

  return (
    <div className="fixed inset-0 z-[90] flex items-center justify-center bg-slate-950/55 px-4 backdrop-blur-sm">
      <div className="w-full max-w-5xl rounded-3xl border border-slate-200/70 bg-white p-6 shadow-2xl dark:border-slate-700/70 dark:bg-slate-900">
        <div className="flex items-start justify-between gap-4">
          <div>
            <h2 className="text-xl font-semibold text-slate-900 dark:text-slate-100">{team ? '编辑团队' : '新建团队'}</h2>
            <p className="mt-1 text-sm text-slate-500 dark:text-slate-400">设置团队名称、主管和执行成员，让后续任务能够更准确地协作与分工。</p>
          </div>
          <button onClick={onClose} disabled={saving} className="inline-flex h-10 w-10 items-center justify-center rounded-2xl border border-slate-200 text-slate-500 transition hover:bg-slate-50 disabled:opacity-50 dark:border-slate-700 dark:text-slate-300 dark:hover:bg-slate-800">
            <X size={16} />
          </button>
        </div>

        <div className="mt-6 grid gap-6 xl:grid-cols-[minmax(0,1.1fr)_420px]">
          <div className="space-y-5">
            <div className="grid gap-4 md:grid-cols-2">
              <div>
                <div className="mb-1 text-xs text-slate-500 dark:text-slate-400">团队名称</div>
                <input
                  value={form.name}
                  onChange={event => setForm(current => ({ ...current, name: event.target.value }))}
                  placeholder="例如：产品交付组"
                  className="w-full rounded-2xl border border-slate-200 bg-white px-3 py-2 text-sm dark:border-slate-700 dark:bg-slate-900"
                />
              </div>
              <div>
                <div className="mb-1 text-xs text-slate-500 dark:text-slate-400">团队 Manager</div>
                <select
                  value={form.managerAgentId}
                  onChange={event => setForm(current => ({
                    ...current,
                    managerAgentId: event.target.value,
                    workerAgentIds: current.workerAgentIds.filter(item => item !== event.target.value),
                  }))}
                  className="w-full rounded-2xl border border-slate-200 bg-white px-3 py-2 text-sm dark:border-slate-700 dark:bg-slate-900"
                >
                  {agents.map(agent => <option key={agent.id} value={agent.id}>{agentLabel(agent)}</option>)}
                </select>
                <div className="mt-2 rounded-2xl border border-amber-200 bg-amber-50/80 px-3 py-2 text-xs text-amber-700 dark:border-amber-500/20 dark:bg-amber-500/10 dark:text-amber-200">
                  建议优先选择 AI 公司主管作为团队协调者，便于统一分配任务和汇总结果。
                  {preferredManager?.id && preferredManager.id !== form.managerAgentId ? ` 当前推荐：${agentLabel(preferredManager)}` : ''}
                </div>
              </div>
            </div>

            <div>
              <div className="mb-1 text-xs text-slate-500 dark:text-slate-400">团队说明</div>
              <textarea
                value={form.description}
                onChange={event => setForm(current => ({ ...current, description: event.target.value }))}
                rows={3}
                placeholder="说明这个执行团队适合处理什么类型的工作"
                className="w-full rounded-2xl border border-slate-200 bg-white px-3 py-2 text-sm dark:border-slate-700 dark:bg-slate-900"
              />
            </div>

            <div>
              <div className="mb-1 flex items-center gap-2 text-xs text-slate-500 dark:text-slate-400">
                <Users size={14} />
                Worker 智能体
              </div>
              <div className="rounded-3xl border border-slate-200/70 bg-slate-50/70 p-4 dark:border-slate-700/70 dark:bg-slate-950/40">
                <div className="flex flex-wrap gap-2">
                  {availableWorkers.map(agent => {
                    const active = form.workerAgentIds.includes(agent.id);
                    return (
                      <button
                        key={agent.id}
                        type="button"
                        onClick={() => handleToggleWorker(agent.id)}
                        className={`rounded-full px-3 py-1.5 text-xs transition ${active ? 'border border-blue-200 bg-blue-50 text-blue-700 dark:border-blue-500/20 dark:bg-blue-500/10 dark:text-blue-200' : 'border border-slate-200 bg-white text-slate-500 dark:border-slate-700 dark:bg-slate-900 dark:text-slate-300'}`}
                      >
                        {agentLabel(agent)}
                      </button>
                    );
                  })}
                </div>
              </div>
              <div className="mt-3 rounded-2xl border border-slate-200/70 bg-white/80 px-3 py-2 text-xs text-slate-500 dark:border-slate-700/70 dark:bg-slate-900/70 dark:text-slate-400">
                本弹窗用于设置团队成员关系；如需补充账号、通道或路由配置，可前往智能体与通道设置页面继续完善。
              </div>
            </div>

            {error && <div className="rounded-2xl border border-rose-200 bg-rose-50 px-3 py-2 text-sm text-rose-700 dark:border-rose-500/30 dark:bg-rose-500/10 dark:text-rose-200">{error}</div>}
          </div>

          <CompanyOrgChart
            team={previewTeam}
            title="团队预览"
            subtitle="预览团队中的主管与执行成员关系。"
            compact
            hideIdleBadge
          />
        </div>

        <div className="mt-6 flex items-center justify-end gap-3">
          <button onClick={onClose} disabled={saving} className="rounded-2xl border border-slate-200 px-4 py-2 text-sm font-medium text-slate-700 transition hover:bg-slate-50 disabled:opacity-50 dark:border-slate-700 dark:text-slate-200 dark:hover:bg-slate-800">取消</button>
          <button onClick={() => void handleSubmit()} disabled={saving} className="inline-flex items-center gap-2 rounded-2xl bg-slate-900 px-4 py-2 text-sm font-medium text-white transition hover:bg-slate-800 disabled:opacity-50 dark:bg-white dark:text-slate-900 dark:hover:bg-slate-100">
            {saving ? <Loader2 className="animate-spin" size={16} /> : <Plus size={16} />}
            {team ? '保存团队' : '创建团队'}
          </button>
        </div>
      </div>
    </div>
  );
}
