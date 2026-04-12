import { useEffect, useMemo, useState } from 'react';
import { Bot, Radio, Route } from 'lucide-react';
import { Link } from 'react-router-dom';
import {
  CompanyBindingDuplicateWarning,
  CompanyAgentAccessSummary,
  CompanyAgentOption,
  CompanyBindingRecord,
  CompanyChannelConfigMeta,
  CompanyTeamMember,
  agentLabel,
  buildCompanyChannelMeta,
  humanizeCompanyChannelName,
} from './types';

type Props = {
  member: CompanyTeamMember | null;
  agentOption?: CompanyAgentOption | null;
  accessSummary?: CompanyAgentAccessSummary;
  channelsRaw: Record<string, unknown>;
  bindings: CompanyBindingRecord[];
  duplicateWarnings?: CompanyBindingDuplicateWarning[];
  onStageAccountChange?: (agentId: string, channelId: string, accountId: string) => void;
  hasPendingChanges?: boolean;
};

export default function CompanyAgentInfoCard({
  member,
  agentOption,
  accessSummary,
  channelsRaw,
  bindings,
  duplicateWarnings = [],
  onStageAccountChange,
  hasPendingChanges = false,
}: Props) {
  const channelMeta = useMemo(() => buildCompanyChannelMeta(channelsRaw), [channelsRaw]);
  const candidateChannels = useMemo(() => {
    const fromSummary = (accessSummary?.channels || []).map(channel => channel.id);
    const configured = Object.values(channelMeta)
      .filter(channel => channel.enabled && channel.accounts.length > 0)
      .map(channel => channel.id);
    return Array.from(new Set([...fromSummary.filter(Boolean), ...configured]));
  }, [accessSummary?.channels, channelMeta]);
  const [selectedChannelId, setSelectedChannelId] = useState('');
  const [selectedAccountId, setSelectedAccountId] = useState('');

  useEffect(() => {
    const nextChannelId = candidateChannels[0] || '';
    setSelectedChannelId(nextChannelId);
    setSelectedAccountId(channelMeta[nextChannelId]?.defaultAccount || '');
  }, [candidateChannels, channelMeta, member?.agentId]);

  useEffect(() => {
    if (!member?.agentId || !selectedChannelId) return;
    const matched = bindings.find(binding => {
      const match = binding.match || {};
      return binding.agentId === member.agentId && match.channel === selectedChannelId;
    });
    const nextAccount = String(matched?.match?.accountId || '').trim() || channelMeta[selectedChannelId]?.defaultAccount || '';
    setSelectedAccountId(nextAccount);
  }, [bindings, channelMeta, member?.agentId, selectedChannelId]);

  const selectedChannelMeta: CompanyChannelConfigMeta | undefined = selectedChannelId ? channelMeta[selectedChannelId] : undefined;
  const selectedChannelSummary = accessSummary?.channels.find(channel => channel.id === selectedChannelId);
  const accountOptions = selectedChannelMeta?.accounts || [];
  const duplicateWarning = duplicateWarnings.find(item => item.channelId === selectedChannelId && item.accountId === selectedAccountId);
  const currentAccountText = selectedChannelSummary?.explicitAccounts.length
    ? selectedChannelSummary.explicitAccounts.join(', ')
    : (selectedChannelMeta?.defaultAccount ? `默认账号 (${selectedChannelMeta.defaultAccount})` : '默认账号');

  if (!member) {
    return (
      <div className="rounded-3xl border border-dashed border-slate-300/80 bg-slate-50/70 p-5 text-sm text-slate-500 dark:border-slate-700 dark:bg-slate-900/40 dark:text-slate-400">
        点击协作结构图中的智能体，可查看该成员的基础信息与通道账号切换。
      </div>
    );
  }
  return (
    <div className="rounded-3xl border border-slate-200/70 bg-white/90 p-5 shadow-sm dark:border-slate-700/70 dark:bg-slate-900/85">
      <div className="flex items-start justify-between gap-4">
        <div className="flex items-start gap-3">
          <div className="flex h-12 w-12 items-center justify-center rounded-2xl bg-slate-100 text-slate-700 dark:bg-slate-800 dark:text-slate-200">
            <Bot size={22} />
          </div>
          <div>
            <div className="text-lg font-semibold text-slate-900 dark:text-slate-100">{agentLabel(member)}</div>
            <div className="mt-1 text-sm text-slate-500 dark:text-slate-400">{member.roleType === 'manager' ? 'Manager' : 'Worker'} · {member.dutyLabel || '执行'}</div>
          </div>
        </div>
        <div className="rounded-full bg-slate-100 px-2.5 py-1 text-[11px] text-slate-600 dark:bg-slate-800 dark:text-slate-300">route {accessSummary?.routeCount || 0}</div>
      </div>

      <div className="mt-4 grid gap-3 sm:grid-cols-2">
        <div className="rounded-2xl bg-slate-50 px-4 py-3 dark:bg-slate-800/60">
          <div className="text-xs text-slate-500 dark:text-slate-400">Agent ID</div>
          <div className="mt-1 font-mono text-sm text-slate-900 dark:text-slate-100">{member.agentId}</div>
        </div>
        <div className="rounded-2xl bg-slate-50 px-4 py-3 dark:bg-slate-800/60">
          <div className="text-xs text-slate-500 dark:text-slate-400">通道覆盖</div>
          <div className="mt-1 text-sm text-slate-900 dark:text-slate-100">{accessSummary?.channelCount || 0} 个通道</div>
        </div>
      </div>

      <div className="mt-5 rounded-2xl border border-slate-200/70 px-4 py-4 dark:border-slate-700/70">
        <div className="flex items-center gap-2 text-sm font-medium text-slate-900 dark:text-slate-100">
          <Radio size={15} /> 更换通道账号
        </div>
        <p className="mt-2 text-xs leading-5 text-slate-500 dark:text-slate-400">用于调整该智能体在某个通道下命中的显式 routing 规则账号。若当前还没有显式绑定，暂存时会直接为该智能体新增一条显式通道绑定。</p>

        <div className="mt-4 grid gap-3 sm:grid-cols-2">
          <div>
            <div className="mb-1 text-xs text-slate-500 dark:text-slate-400">通道</div>
            <select value={selectedChannelId} onChange={event => {
              const value = event.target.value;
              setSelectedChannelId(value);
              setSelectedAccountId(channelMeta[value]?.defaultAccount || '');
            }} className="w-full rounded-2xl border border-slate-200 bg-white px-3 py-2 text-sm dark:border-slate-700 dark:bg-slate-900">
              {candidateChannels.length === 0 && <option value="">暂无可用通道</option>}
              {candidateChannels.map(channelId => <option key={channelId} value={channelId}>{humanizeCompanyChannelName(channelId)}</option>)}
            </select>
          </div>
          <div>
            <div className="mb-1 text-xs text-slate-500 dark:text-slate-400">账号</div>
            <select value={selectedAccountId} onChange={event => setSelectedAccountId(event.target.value)} className="w-full rounded-2xl border border-slate-200 bg-white px-3 py-2 text-sm dark:border-slate-700 dark:bg-slate-900">
              <option value="">默认账号{selectedChannelMeta?.defaultAccount ? ` (${selectedChannelMeta.defaultAccount})` : ''}</option>
              {accountOptions.map(accountId => <option key={accountId} value={accountId}>{accountId}</option>)}
            </select>
          </div>
        </div>

        <div className="mt-3 flex flex-wrap gap-2">
          <button
            type="button"
            onClick={() => member?.agentId && selectedChannelId && onStageAccountChange?.(member.agentId, selectedChannelId, selectedAccountId)}
            disabled={!member?.agentId || !selectedChannelId}
            className="inline-flex items-center gap-2 rounded-2xl border border-slate-200 px-3 py-2 text-xs font-medium text-slate-700 transition hover:bg-slate-50 disabled:opacity-50 dark:border-slate-700 dark:text-slate-200 dark:hover:bg-slate-800"
          >
            暂存当前修改
          </button>
          {hasPendingChanges && <span className="rounded-full bg-blue-50 px-2.5 py-2 text-[11px] text-blue-700 dark:bg-blue-500/10 dark:text-blue-200">当前页存在未应用修改</span>}
        </div>

        <div className="mt-3 rounded-2xl bg-slate-50 px-3 py-3 text-xs text-slate-500 dark:bg-slate-800/60 dark:text-slate-400">
          当前账号：{currentAccountText}
          {selectedChannelMeta && ` · 可用账号 ${selectedChannelMeta.accounts.length}`}
          {!selectedChannelSummary && selectedChannelId && ' · 当前将新增显式绑定'}
        </div>

        {duplicateWarning && (
          <div className="mt-3 rounded-2xl border border-amber-200 bg-amber-50 px-3 py-2 text-sm text-amber-700 dark:border-amber-500/30 dark:bg-amber-500/10 dark:text-amber-200">
            警告：`{duplicateWarning.channelLabel}` 的账号 `{duplicateWarning.accountId}` 当前同时匹配 {duplicateWarning.agentIds.join('、')}，请调整到一账号只对应一个智能体。
          </div>
        )}

        <div className="mt-4 flex flex-wrap gap-2">
          <Link to={`/agents?agent=${encodeURIComponent(member.agentId)}&view=routing`} className="inline-flex items-center gap-1 rounded-xl border border-slate-200 px-3 py-2 text-xs font-medium text-slate-700 transition hover:bg-slate-50 dark:border-slate-700 dark:text-slate-200 dark:hover:bg-slate-800">
            <Route size={13} /> 去路由页精细编辑
          </Link>
        </div>
      </div>
    </div>
  );
}
