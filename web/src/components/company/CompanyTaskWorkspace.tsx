import { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import { ClipboardList, Crown, Flag, Radio, Route, Sparkles, Users } from 'lucide-react';
import CompanyAgentInfoCard from './CompanyAgentInfoCard';
import CompanyOrgChart from './CompanyOrgChart';
import { CompanyAgentAccessSummary, CompanyAgentOption, CompanyBindingRecord, CompanyTaskRecord, CompanyTeamMember, NormalizedCompanyTeam, agentLabel, agentStatusTone, detectCompanyBindingDuplicates, retargetAgentChannelBindingAccount, serializeCompanyBindingRecords } from './types';
import { api } from '../../lib/api';

type Props = {
  task: CompanyTaskRecord;
  team: NormalizedCompanyTeam | null;
  accessMap?: Record<string, CompanyAgentAccessSummary>;
  bindings?: CompanyBindingRecord[];
  channelsRaw?: Record<string, unknown>;
  agentOptions?: CompanyAgentOption[];
  onBindingsSaved?: (bindings: CompanyBindingRecord[]) => void;
};

function resolveSummaryActor(task: CompanyTaskRecord, team: NormalizedCompanyTeam | null) {
  if (!task.summaryAgentId?.trim()) return team?.manager || null;
  if (team?.manager?.agentId === task.summaryAgentId) return team.manager;
  return team?.workers.find(worker => worker.agentId === task.summaryAgentId) || ({ agentId: task.summaryAgentId } as CompanyTeamMember);
}

function accessHint(summary?: CompanyAgentAccessSummary) {
  if (!summary) return '尚未配置接入规则';
  if (summary.routeCount > 0) return `${summary.routeCount} 条路由 · ${summary.channelCount} 个通道上下文`;
  if (summary.defaultFallback) return '默认回落 Agent';
  return '尚未配置显式 routing / binding';
}

function listFromInput(value: unknown) {
  if (!Array.isArray(value)) return [] as string[];
  return value.map(item => String(item || '').trim()).filter(Boolean);
}

export default function CompanyTaskWorkspace({ task, team, accessMap = {}, bindings = [], channelsRaw = {}, agentOptions = [], onBindingsSaved }: Props) {
  const summaryActor = resolveSummaryActor(task, team);
  const steps = task.steps || [];
  const events = task.events || [];
  const activeWorkers = team?.workers.filter(worker => steps.some(step => step.workerAgentId === worker.agentId)) || [];
  const relatedMembers = [team?.manager, ...activeWorkers, summaryActor].filter((value, index, list): value is CompanyTeamMember => Boolean(value) && list.findIndex(item => item?.agentId === value?.agentId) === index);
  const [selectedMember, setSelectedMember] = useState<CompanyTeamMember | null>(team?.manager || relatedMembers[0] || null);
  const [draftBindings, setDraftBindings] = useState<CompanyBindingRecord[]>(bindings);
  const duplicateWarnings = detectCompanyBindingDuplicates(draftBindings, channelsRaw);
  const hasPendingBindingChanges = JSON.stringify(serializeCompanyBindingRecords(draftBindings)) !== JSON.stringify(serializeCompanyBindingRecords(bindings));
  const usedFallbackSummary = task.reviewComment === 'fallback summary' || task.reviewComment === 'fallback summary from worker outputs' || task.reviewComment === 'review fallback accepted';

  useEffect(() => {
    setSelectedMember(team?.manager || relatedMembers[0] || null);
  }, [team?.manager?.agentId, relatedMembers]);

  useEffect(() => {
    setDraftBindings(bindings);
  }, [bindings]);

  const stageAccountChange = (agentId: string, channelId: string, accountId: string) => {
    const { nextBindings } = retargetAgentChannelBindingAccount(draftBindings, agentId, channelId, accountId);
    setDraftBindings(nextBindings);
  };

  const applyBindingChanges = async () => {
    const res = await api.updateBindings(serializeCompanyBindingRecords(draftBindings));
    if (!res?.ok) return;
    onBindingsSaved?.(draftBindings);
  };

  return (
    <div className="space-y-6">
      <CompanyOrgChart
        team={team}
        task={task}
        title="真实协作关系"
        subtitle="查看当前任务中主管与执行成员之间的协作分工。"
        selectedAgentId={selectedMember?.agentId}
        onSelectAgent={setSelectedMember}
      />

      <CompanyAgentInfoCard
        member={selectedMember}
        agentOption={agentOptions.find(agent => agent.id === selectedMember?.agentId) || null}
        accessSummary={selectedMember ? accessMap[selectedMember.agentId] : undefined}
        channelsRaw={channelsRaw}
        bindings={draftBindings}
        duplicateWarnings={duplicateWarnings}
        onStageAccountChange={stageAccountChange}
        hasPendingChanges={hasPendingBindingChanges}
      />
      <div className="flex flex-wrap gap-2">
        <button onClick={() => void applyBindingChanges()} disabled={!hasPendingBindingChanges} className="inline-flex items-center gap-2 rounded-2xl bg-slate-900 px-4 py-2 text-sm font-medium text-white transition hover:bg-slate-800 disabled:opacity-50 dark:bg-white dark:text-slate-900 dark:hover:bg-slate-100">
          应用账号切换
        </button>
        <Link to="/agents?view=routing" className="inline-flex items-center gap-1 rounded-xl border border-slate-200 px-3 py-2 text-xs font-medium text-slate-700 transition hover:bg-slate-50 dark:border-slate-700 dark:text-slate-200 dark:hover:bg-slate-800">
          <Route size={13} /> 去路由页精细编辑
        </Link>
      </div>

      <div className="grid gap-4 xl:grid-cols-3">
        <section className="rounded-3xl border border-amber-200/70 bg-[linear-gradient(180deg,rgba(255,251,235,0.96),rgba(255,255,255,0.96))] p-5 shadow-sm dark:border-amber-500/20 dark:bg-[linear-gradient(180deg,rgba(120,53,15,0.18),rgba(15,23,42,0.92))]">
          <div className="flex items-center gap-3 text-amber-700 dark:text-amber-200">
            <Crown size={18} />
            <h3 className="text-base font-semibold">主管协调</h3>
          </div>
          <div className="mt-4 text-sm text-slate-700 dark:text-slate-200">
            <div className="font-medium text-slate-900 dark:text-slate-100">{team?.manager ? agentLabel(team.manager) : '未识别 Manager'}</div>
            <p className="mt-2 leading-6 text-slate-600 dark:text-slate-300">负责理解任务目标、拆解工作、协调执行成员，并在完成后整理最终结果。</p>
          </div>
          <div className="mt-4 flex flex-wrap gap-2 text-xs text-slate-500 dark:text-slate-400">
            <span className="rounded-full bg-white/80 px-2.5 py-1 shadow-sm dark:bg-slate-950/60">来源 {task.sourceType || 'panel_manual'}</span>
            {(task.sourceChannelType || task.sourceChannelId) && <span className="rounded-full bg-white/80 px-2.5 py-1 shadow-sm dark:bg-slate-950/60">{task.sourceChannelType || 'source'} {task.sourceChannelId || ''}</span>}
            <span className="rounded-full bg-white/80 px-2.5 py-1 shadow-sm dark:bg-slate-950/60">投递 {task.deliveryType || 'notify_only'}</span>
            {(task.deliveryChannelType || task.deliveryChannelId) && <span className="rounded-full bg-white/80 px-2.5 py-1 shadow-sm dark:bg-slate-950/60">{task.deliveryChannelType || 'delivery'} {task.deliveryChannelId || ''}</span>}
          </div>
        </section>

        <section className="rounded-3xl border border-blue-200/70 bg-[linear-gradient(180deg,rgba(239,246,255,0.96),rgba(255,255,255,0.96))] p-5 shadow-sm dark:border-blue-500/20 dark:bg-[linear-gradient(180deg,rgba(30,64,175,0.12),rgba(15,23,42,0.92))]">
          <div className="flex items-center gap-3 text-blue-700 dark:text-blue-200">
            <Users size={18} />
            <h3 className="text-base font-semibold">成员执行</h3>
          </div>
          <div className="mt-4 space-y-3">
            {activeWorkers.length === 0 && (
              <div className="rounded-2xl border border-dashed border-blue-200/80 bg-white/70 px-4 py-4 text-sm text-slate-500 dark:border-blue-500/20 dark:bg-slate-950/40 dark:text-slate-400">
                当前任务还没有识别到具体 Worker 分工。
              </div>
            )}
            {activeWorkers.map(worker => {
              const assigned = steps.filter(step => step.workerAgentId === worker.agentId);
              const running = assigned.some(step => step.status === 'running');
              const failed = assigned.some(step => step.status === 'failed');
              const status = failed ? 'failed' : running ? 'running' : assigned.length > 0 ? 'completed' : '';
              return (
                <div key={worker.agentId} className="rounded-2xl border border-blue-200/70 bg-white/80 px-4 py-4 dark:border-blue-500/20 dark:bg-slate-950/40">
                  <div className="flex items-start justify-between gap-3">
                    <div>
                      <div className="font-medium text-slate-900 dark:text-slate-100">{agentLabel(worker)}</div>
                      <div className="mt-1 text-xs text-slate-500 dark:text-slate-400">{worker.dutyLabel || '执行'} · {assigned.length} 个步骤</div>
                    </div>
                    <div className={`rounded-full px-2.5 py-1 text-[11px] font-medium ${agentStatusTone(status)}`}>
                      {status || '待命'}
                    </div>
                  </div>
                </div>
              );
            })}
          </div>
        </section>

        <section className="rounded-3xl border border-emerald-200/70 bg-[linear-gradient(180deg,rgba(236,253,245,0.96),rgba(255,255,255,0.96))] p-5 shadow-sm dark:border-emerald-500/20 dark:bg-[linear-gradient(180deg,rgba(6,78,59,0.14),rgba(15,23,42,0.92))]">
          <div className="flex items-center gap-3 text-emerald-700 dark:text-emerald-200">
            <Sparkles size={18} />
            <h3 className="text-base font-semibold">结果汇总</h3>
          </div>
          <div className="mt-4 text-sm text-slate-700 dark:text-slate-200">
            <div className="font-medium text-slate-900 dark:text-slate-100">{summaryActor ? agentLabel(summaryActor) : '未识别汇总角色'}</div>
            <p className="mt-2 leading-6 text-slate-600 dark:text-slate-300">默认由主管统一审核并整理结果；如果任务使用了历史兼容配置，这里会展示实际负责汇总的成员。</p>
          </div>
          <div className="mt-4 rounded-2xl border border-emerald-200/70 bg-white/80 px-4 py-4 text-sm leading-6 text-slate-700 dark:border-emerald-500/20 dark:bg-slate-950/40 dark:text-slate-200">
            {task.resultText?.trim() || '任务尚未产出最终结果。'}
          </div>
          {(task.reviewResult || task.reviewComment) && (
            <div className="mt-3 flex flex-wrap gap-2 text-xs">
              {task.reviewResult && <span className="rounded-full bg-white/80 px-2.5 py-1 text-slate-600 shadow-sm dark:bg-slate-950/60 dark:text-slate-300">审核 {task.reviewResult}</span>}
              {task.reviewComment && <span className="rounded-full bg-white/80 px-2.5 py-1 text-slate-600 shadow-sm dark:bg-slate-950/60 dark:text-slate-300">{task.reviewComment}</span>}
            </div>
          )}
          {usedFallbackSummary && (
            <div className="mt-3 rounded-2xl border border-amber-200 bg-amber-50 px-3 py-2 text-xs leading-5 text-amber-700 dark:border-amber-500/30 dark:bg-amber-500/10 dark:text-amber-200">
              当前结果由系统自动整理生成，方便你快速查看整体进展；如需更完整信息，可结合下方执行步骤与事件记录一起查看。
            </div>
          )}
        </section>
      </div>

      <section className="rounded-3xl border border-slate-200/70 bg-white/90 p-6 shadow-sm dark:border-slate-700/70 dark:bg-slate-900/85">
        <div className="mb-4 flex items-center gap-3">
          <ClipboardList size={18} className="text-slate-500 dark:text-slate-400" />
          <h3 className="text-lg font-semibold text-slate-900 dark:text-slate-100">执行步骤</h3>
        </div>
        <div className="space-y-3">
          {steps.length === 0 && (
            <div className="rounded-2xl border border-dashed border-slate-300/80 bg-slate-50/70 px-4 py-5 text-sm text-slate-500 dark:border-slate-700 dark:bg-slate-900/40 dark:text-slate-400">
              还没有生成步骤计划。
            </div>
          )}
          {steps.map(step => (
            <div key={step.id} className="rounded-2xl border border-slate-200/70 px-4 py-4 dark:border-slate-700/70">
              <div className="flex items-start justify-between gap-4">
                <div>
                  <div className="font-medium text-slate-900 dark:text-slate-100">{step.title}</div>
                  <div className="mt-1 text-xs text-slate-500 dark:text-slate-400">{step.stepKey} · {step.workerAgentId || '未分配'} · {step.status}</div>
                </div>
                <div className={`rounded-full px-2.5 py-1 text-[11px] font-medium ${agentStatusTone(step.status)}`}>
                  {step.status}
                </div>
              </div>
              <div className="mt-3 rounded-2xl bg-slate-50/80 px-4 py-3 text-sm leading-6 text-slate-700 dark:bg-slate-800/60 dark:text-slate-200">
                {step.outputText?.trim() || step.errorText?.trim() || step.instruction?.trim() || '暂无内容'}
              </div>
              <div className="mt-3 flex flex-wrap gap-2 text-[11px] text-slate-500 dark:text-slate-400">
                {listFromInput(step.input?.requiredSkills).map(skill => <span key={`${step.id}-skill-${skill}`} className="rounded-full bg-slate-100 px-2 py-1 dark:bg-slate-800">skill {skill}</span>)}
                {listFromInput(step.input?.expectedArtifacts).map(artifact => <span key={`${step.id}-artifact-${artifact}`} className="rounded-full bg-blue-50 px-2 py-1 text-blue-700 dark:bg-blue-500/10 dark:text-blue-200">artifact {artifact}</span>)}
                {listFromInput(step.input?.dependsOn).map(dep => <span key={`${step.id}-dep-${dep}`} className="rounded-full bg-amber-50 px-2 py-1 text-amber-700 dark:bg-amber-500/10 dark:text-amber-200">dependsOn {dep}</span>)}
                {Boolean(step.input?.assignmentReason) && <span className="rounded-full bg-emerald-50 px-2 py-1 text-emerald-700 dark:bg-emerald-500/10 dark:text-emerald-200">{String(step.input?.assignmentReason)}</span>}
              </div>
            </div>
          ))}
        </div>
      </section>

      <section className="rounded-3xl border border-slate-200/70 bg-white/90 p-6 shadow-sm dark:border-slate-700/70 dark:bg-slate-900/85">
        <div className="mb-4 flex items-center gap-3">
          <Flag size={18} className="text-slate-500 dark:text-slate-400" />
          <h3 className="text-lg font-semibold text-slate-900 dark:text-slate-100">执行事件流</h3>
        </div>
        <div className="space-y-3">
          {events.length === 0 && (
            <div className="rounded-2xl border border-dashed border-slate-300/80 bg-slate-50/70 px-4 py-5 text-sm text-slate-500 dark:border-slate-700 dark:bg-slate-900/40 dark:text-slate-400">
              暂无执行事件。
            </div>
          )}
          {events.map(event => (
            <div key={event.id} className="rounded-2xl border border-slate-200/70 px-4 py-3 dark:border-slate-700/70">
              <div className="flex items-start justify-between gap-4">
                <div>
                  <div className="font-medium text-slate-900 dark:text-slate-100">{event.message}</div>
                  <div className="mt-1 text-xs text-slate-500 dark:text-slate-400">{event.eventType}{event.stepId ? ` · ${event.stepId}` : ''}</div>
                </div>
                <div className="text-xs text-slate-400 dark:text-slate-500">{new Date(event.createdAt).toLocaleString()}</div>
              </div>
            </div>
          ))}
        </div>
      </section>

      <section className="rounded-3xl border border-slate-200/70 bg-white/90 p-6 shadow-sm dark:border-slate-700/70 dark:bg-slate-900/85">
        <div className="mb-4 flex items-center gap-3">
          <Radio size={18} className="text-slate-500 dark:text-slate-400" />
          <h3 className="text-lg font-semibold text-slate-900 dark:text-slate-100">成员接入摘要</h3>
        </div>
        <p className="text-sm text-slate-500 dark:text-slate-400">这里只读展示相关成员的接入覆盖情况；真实编辑入口仍在智能体路由与全局通道配置页面。</p>
        <div className="mt-4 grid gap-3 md:grid-cols-2 xl:grid-cols-3">
          {relatedMembers.map(member => {
            const summary = accessMap[member.agentId];
            const firstChannel = summary?.channels?.[0]?.id;
            const roleLabel = member.agentId === team?.manager?.agentId ? 'Manager' : member.agentId === summaryActor?.agentId ? 'Summary' : 'Worker';
            return (
              <div key={member.agentId} className="rounded-2xl border border-slate-200/70 px-4 py-4 dark:border-slate-700/70">
                <div className="flex items-start justify-between gap-3">
                  <div>
                    <div className="font-medium text-slate-900 dark:text-slate-100">{agentLabel(member)}</div>
                    <div className="mt-1 text-xs text-slate-500 dark:text-slate-400">{roleLabel} · {accessHint(summary)}</div>
                  </div>
                  <div className="rounded-full bg-slate-100 px-2.5 py-1 text-[11px] text-slate-600 dark:bg-slate-800 dark:text-slate-300">route {summary?.routeCount || 0}</div>
                </div>
                <div className="mt-3 flex flex-wrap gap-2 text-[11px] text-slate-500 dark:text-slate-400">
                  {(summary?.channels || []).length > 0 ? summary?.channels.slice(0, 3).map(channel => (
                    <span key={`${member.agentId}-${channel.id}`} className="rounded-full bg-slate-100 px-2.5 py-1 dark:bg-slate-800">
                      {channel.label}
                      {channel.explicitAccounts.length > 0 ? ` · ${channel.explicitAccounts.join(', ')}` : channel.defaultAccount ? ` · 默认账号 ${channel.defaultAccount}` : ''}
                    </span>
                  )) : <span className="rounded-full bg-slate-100 px-2.5 py-1 dark:bg-slate-800">尚无显式通道绑定</span>}
                </div>
                <div className="mt-4 flex flex-wrap gap-2">
                  <Link to={`/agents?agent=${encodeURIComponent(member.agentId)}&view=routing`} className="inline-flex items-center gap-1 rounded-xl border border-slate-200 px-3 py-1.5 text-xs font-medium text-slate-700 transition hover:bg-slate-50 dark:border-slate-700 dark:text-slate-200 dark:hover:bg-slate-800"><Route size={13} /> 查看路由规则</Link>
                  <Link to={firstChannel ? `/channels?channel=${encodeURIComponent(firstChannel)}` : '/channels'} className="inline-flex items-center gap-1 rounded-xl border border-slate-200 px-3 py-1.5 text-xs font-medium text-slate-700 transition hover:bg-slate-50 dark:border-slate-700 dark:text-slate-200 dark:hover:bg-slate-800"><Radio size={13} /> 查看通道配置</Link>
                </div>
              </div>
            );
          })}
        </div>
      </section>
    </div>
  );
}
