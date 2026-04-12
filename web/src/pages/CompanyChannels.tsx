import { useEffect, useMemo, useState } from 'react';
import { Link, useOutletContext } from 'react-router-dom';
import { Loader2, Radio, Route, Settings2 } from 'lucide-react';
import {
  CompanyAgentOption,
  CompanyBindingRecord,
  buildCompanyAgentAccessMap,
  buildCompanyAgentOptions,
  countConfiguredCompanyChannels,
} from '../components/company/types';
import { api } from '../lib/api';

export default function CompanyChannels() {
  const { uiMode } = (useOutletContext() as { uiMode?: 'modern' }) || {};
  const [channels, setChannels] = useState<any[]>([]);
  const [capabilities, setCapabilities] = useState<any>(null);
  const [agents, setAgents] = useState<CompanyAgentOption[]>([]);
  const [bindings, setBindings] = useState<CompanyBindingRecord[]>([]);
  const [defaultAgentId, setDefaultAgentId] = useState('main');
  const [channelConfigs, setChannelConfigs] = useState<Record<string, unknown>>({});
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    (async () => {
      try {
        const [channelRes, capabilityRes, agentsRes, channelsConfigRes] = await Promise.all([
          api.getCompanyChannels(),
          api.getCompanyCapabilities(),
          api.getAgentsConfig(),
          api.getChannels(),
        ]);
        if (channelRes?.ok) setChannels(channelRes.channels || []);
        if (capabilityRes?.ok) setCapabilities(capabilityRes || null);
        if (agentsRes?.ok) {
          setAgents(buildCompanyAgentOptions(agentsRes?.agents?.list));
          setBindings(Array.isArray(agentsRes?.agents?.bindings) ? agentsRes.agents.bindings : []);
          setDefaultAgentId(String(agentsRes?.agents?.default || 'main').trim() || 'main');
        }
        if (channelsConfigRes?.ok) setChannelConfigs((channelsConfigRes.channels || {}) as Record<string, unknown>);
      } finally {
        setLoading(false);
      }
    })();
  }, []);

  const agentIds = useMemo(() => agents.map(agent => agent.id), [agents]);
  const accessMap = useMemo(() => buildCompanyAgentAccessMap(agentIds, bindings, channelConfigs, defaultAgentId), [agentIds, bindings, channelConfigs, defaultAgentId]);
  const configuredChannelCount = useMemo(() => countConfiguredCompanyChannels(channelConfigs), [channelConfigs]);
  const routedAgentCount = useMemo(() => agentIds.filter(agentId => (accessMap[agentId]?.routeCount || 0) > 0).length, [accessMap, agentIds]);
  const totalRouteCount = useMemo(() => agentIds.reduce((sum, agentId) => sum + (accessMap[agentId]?.routeCount || 0), 0), [accessMap, agentIds]);

  return (
    <div className={uiMode === 'modern' ? 'space-y-6' : 'space-y-4'}>
      <div className="rounded-3xl border border-slate-200/70 bg-white/90 p-6 shadow-sm dark:border-slate-700/70 dark:bg-slate-900/85">
        <div className="inline-flex items-center gap-2 rounded-full border border-slate-200 bg-slate-50 px-3 py-1 text-xs font-medium text-slate-700 dark:border-slate-700 dark:bg-slate-800/70 dark:text-slate-200">
          <Settings2 size={14} />
          接入设置
        </div>
        <h1 className="mt-3 text-2xl font-semibold text-slate-900 dark:text-slate-100">接入设置</h1>
        <p className="mt-2 text-sm text-slate-500 dark:text-slate-400">在这里查看当前可用的接入方式，并进入账号与路由配置页面，为团队协作做好准备。</p>
      </div>

      {loading ? (
        <div className="flex min-h-[240px] items-center justify-center rounded-3xl border border-slate-200/70 bg-white/90 dark:border-slate-700/70 dark:bg-slate-900/85">
          <Loader2 className="mr-2 animate-spin" size={18} /> 加载中...
        </div>
      ) : (
        <>
          <div className="grid gap-4 xl:grid-cols-3">
            <div className="rounded-3xl border border-slate-200/70 bg-white/90 p-6 shadow-sm dark:border-slate-700/70 dark:bg-slate-900/85">
              <div className="text-xs text-slate-500 dark:text-slate-400">已配置通道数</div>
              <div className="mt-2 text-2xl font-semibold text-slate-900 dark:text-slate-100">{configuredChannelCount}</div>
              <p className="mt-2 text-sm text-slate-500 dark:text-slate-400">表示当前已经配置好的外部接入渠道数量，决定哪些应用账号可以真正接入系统。</p>
            </div>
            <div className="rounded-3xl border border-slate-200/70 bg-white/90 p-6 shadow-sm dark:border-slate-700/70 dark:bg-slate-900/85">
              <div className="text-xs text-slate-500 dark:text-slate-400">有路由规则的 Agent</div>
              <div className="mt-2 text-2xl font-semibold text-slate-900 dark:text-slate-100">{routedAgentCount}</div>
              <p className="mt-2 text-sm text-slate-500 dark:text-slate-400">表示当前已有明确分工的智能体数量，决定外部消息会落到谁来处理。</p>
            </div>
            <div className="rounded-3xl border border-slate-200/70 bg-white/90 p-6 shadow-sm dark:border-slate-700/70 dark:bg-slate-900/85">
              <div className="text-xs text-slate-500 dark:text-slate-400">显式路由规则</div>
              <div className="mt-2 text-2xl font-semibold text-slate-900 dark:text-slate-100">{totalRouteCount}</div>
              <p className="mt-2 text-sm text-slate-500 dark:text-slate-400">表示当前已经生效的路由规则数量，帮助你判断不同账号是否已经完成独立分流。</p>
            </div>
          </div>

          <div className="grid gap-6 xl:grid-cols-[minmax(0,1fr)_360px]">
            <div className="rounded-3xl border border-slate-200/70 bg-white/90 p-6 shadow-sm dark:border-slate-700/70 dark:bg-slate-900/85">
              <h2 className="text-lg font-semibold text-slate-900 dark:text-slate-100">去配置入口</h2>
              <div className="mt-4 grid gap-3 md:grid-cols-2">
                <Link to="/channels" className="rounded-3xl border border-slate-200/70 px-5 py-5 transition hover:bg-slate-50 dark:border-slate-700/70 dark:hover:bg-slate-800/70">
                  <div className="flex items-center gap-3 text-slate-900 dark:text-slate-100">
                    <Radio size={18} />
                    <div className="font-medium">通道配置</div>
                  </div>
                  <p className="mt-2 text-sm text-slate-500 dark:text-slate-400">在这里配置应用账号、默认账号以及各渠道的接入参数。</p>
                </Link>
                <Link to="/agents?view=routing" className="rounded-3xl border border-slate-200/70 px-5 py-5 transition hover:bg-slate-50 dark:border-slate-700/70 dark:hover:bg-slate-800/70">
                  <div className="flex items-center gap-3 text-slate-900 dark:text-slate-100">
                    <Route size={18} />
                    <div className="font-medium">Agent 路由规则</div>
                  </div>
                  <p className="mt-2 text-sm text-slate-500 dark:text-slate-400">在这里把不同渠道和账号分配给不同智能体，并验证消息会路由到谁。</p>
                </Link>
              </div>
            </div>

            <div className="rounded-3xl border border-slate-200/70 bg-white/90 p-6 shadow-sm dark:border-slate-700/70 dark:bg-slate-900/85">
              <h2 className="text-lg font-semibold text-slate-900 dark:text-slate-100">如何使用</h2>
              <div className="mt-4 space-y-3 text-sm leading-6 text-slate-600 dark:text-slate-300">
                <p>1. 先在这里配置可用账号与通道，再将不同成员分配到合适的接入账号。</p>
                <p>2. 团队页面会展示成员当前的接入情况，方便你快速检查是否配置完成。</p>
                <p>3. 已接通的账号可以立即用于协作任务和消息分流。</p>
              </div>
            </div>
          </div>

          <div className="grid gap-6 xl:grid-cols-[minmax(0,1fr)_320px]">
            <div className="rounded-3xl border border-slate-200/70 bg-white/90 p-6 shadow-sm dark:border-slate-700/70 dark:bg-slate-900/85">
              <h2 className="text-lg font-semibold text-slate-900 dark:text-slate-100">当前通道状态</h2>
              <div className="mt-4 space-y-3">
                {channels.map(channel => (
                  <div key={channel.channelType} className="rounded-2xl border border-slate-200/70 px-4 py-4 dark:border-slate-700/70">
                    <div className="flex items-center justify-between gap-4">
                      <div className="flex items-center gap-3">
                        <div className="flex h-9 w-9 items-center justify-center rounded-full bg-slate-100 text-slate-600 dark:bg-slate-800 dark:text-slate-300"><Radio size={16} /></div>
                        <div>
                          <div className="font-medium text-slate-900 dark:text-slate-100">{channel.label}</div>
                          <div className="mt-1 text-xs text-slate-500 dark:text-slate-400">{channel.channelType}</div>
                        </div>
                      </div>
                      <div className="rounded-full bg-slate-100 px-2.5 py-1 text-xs text-slate-600 dark:bg-slate-800 dark:text-slate-300">{channel.status}</div>
                    </div>
                    <div className="mt-3 text-sm text-slate-600 dark:text-slate-300">{channel.note || '该通道已预留，等待后续开放。'}</div>
                    <div className="mt-3 flex gap-2 text-xs">
                      <span className={`rounded-full px-2 py-1 ${channel.sourceReady ? 'bg-emerald-50 text-emerald-700 dark:bg-emerald-500/10 dark:text-emerald-200' : 'bg-slate-100 text-slate-600 dark:bg-slate-800 dark:text-slate-300'}`}>source {channel.sourceReady ? 'ready' : 'reserved'}</span>
                      <span className={`rounded-full px-2 py-1 ${channel.deliveryReady ? 'bg-blue-50 text-blue-700 dark:bg-blue-500/10 dark:text-blue-200' : 'bg-slate-100 text-slate-600 dark:bg-slate-800 dark:text-slate-300'}`}>delivery {channel.deliveryReady ? 'ready' : 'reserved'}</span>
                    </div>
                  </div>
                ))}
              </div>
            </div>

            <div className="rounded-3xl border border-slate-200/70 bg-white/90 p-6 shadow-sm dark:border-slate-700/70 dark:bg-slate-900/85">
              <h2 className="text-lg font-semibold text-slate-900 dark:text-slate-100">任务支持范围</h2>
              <div className="mt-4">
                <div className="text-xs text-slate-500 dark:text-slate-400">source_type</div>
                <div className="mt-2 flex flex-wrap gap-2">
                  {(capabilities?.sourceTypes || []).map((item: string) => <span key={item} className="rounded-full bg-slate-100 px-2 py-1 text-xs text-slate-600 dark:bg-slate-800 dark:text-slate-300">{item}</span>)}
                </div>
              </div>
              <div className="mt-5">
                <div className="text-xs text-slate-500 dark:text-slate-400">delivery_type</div>
                <div className="mt-2 flex flex-wrap gap-2">
                  {(capabilities?.deliveryTypes || []).map((item: string) => <span key={item} className="rounded-full bg-slate-100 px-2 py-1 text-xs text-slate-600 dark:bg-slate-800 dark:text-slate-300">{item}</span>)}
                </div>
              </div>
            </div>
          </div>
        </>
      )}
    </div>
  );
}
