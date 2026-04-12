import { Fragment, useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { useOutletContext } from 'react-router-dom';
import { Bot, Check, Copy, Loader2, MessageSquarePlus, Send, Square, Trash2, User, Users } from 'lucide-react';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import { api } from '../lib/api';
import { useI18n } from '../i18n';

type PanelChatSession = {
  id: string;
  openclawSessionId: string;
  agentId: string;
  chatType: 'direct' | 'group';
  title: string;
  targetId?: string;
  targetName?: string;
  createdAt: number;
  updatedAt: number;
  processing?: boolean;
  currentAgentId?: string;
  currentAgentName?: string;
  summaryAgentId?: string;
  messageCount: number;
  lastMessage?: string;
  participantCount?: number;
};

type PanelChatMessage = {
  id: string;
  role: string;
  senderType?: 'user' | 'agent' | 'system';
  agentId?: string;
  agentName?: string;
  messageType?: 'chat' | 'summary' | 'system_notice';
  content: string;
  timestamp: string;
  sessionId?: string;
  images?: { src: string; mimeType?: string }[];
  sources?: { path: string; title?: string; excerpt?: string; score?: number }[];
};

type AgentOption = {
  id: string;
  name?: string;
  isDefault?: boolean;
};

type SessionParticipant = {
  agentId: string;
  name?: string;
  roleType?: string;
  orderIndex?: number;
  autoReply?: boolean;
  enabled?: boolean;
  isSummary?: boolean;
};

type SharedContextItem = {
  path: string;
  title?: string;
};

function normalizeUserMessageContent(content: string) {
  return content.replace(/^\[[^\]]+\]\s*/, '').trim();
}

function agentBadgeTone(agentId: string) {
  const normalized = agentId.trim().toLowerCase();
  const fixed: Record<string, string> = {
    main: 'border border-sky-200 bg-sky-100 text-sky-700 dark:border-sky-500/30 dark:bg-sky-500/15 dark:text-sky-200',
    coding: 'border border-emerald-200 bg-emerald-100 text-emerald-700 dark:border-emerald-500/30 dark:bg-emerald-500/15 dark:text-emerald-200',
    writer: 'border border-violet-200 bg-violet-100 text-violet-700 dark:border-violet-500/30 dark:bg-violet-500/15 dark:text-violet-200',
    reviewer: 'border border-amber-200 bg-amber-100 text-amber-700 dark:border-amber-500/30 dark:bg-amber-500/15 dark:text-amber-200',
  };
  if (fixed[normalized]) return fixed[normalized];
  const tones = [
    'border border-sky-200 bg-sky-100 text-sky-700 dark:border-sky-500/30 dark:bg-sky-500/15 dark:text-sky-200',
    'border border-emerald-200 bg-emerald-100 text-emerald-700 dark:border-emerald-500/30 dark:bg-emerald-500/15 dark:text-emerald-200',
    'border border-amber-200 bg-amber-100 text-amber-700 dark:border-amber-500/30 dark:bg-amber-500/15 dark:text-amber-200',
    'border border-rose-200 bg-rose-100 text-rose-700 dark:border-rose-500/30 dark:bg-rose-500/15 dark:text-rose-200',
  ];
  let hash = 0;
  for (let i = 0; i < agentId.length; i += 1) hash = (hash + agentId.charCodeAt(i)) % tones.length;
  return tones[hash];
}

function agentDisplayName(agent: AgentOption | null | undefined) {
  if (!agent) return '';
  return agent.name ? `${agent.name} (${agent.id})` : agent.id;
}

function sessionBadgeTone(chatType: 'direct' | 'group') {
  return chatType === 'group'
    ? 'border border-emerald-200 bg-emerald-100 text-emerald-700 dark:border-emerald-500/30 dark:bg-emerald-500/15 dark:text-emerald-200'
    : 'border border-slate-200 bg-slate-100 text-slate-700 dark:border-slate-600 dark:bg-slate-800 dark:text-slate-200';
}

export default function PanelChat() {
  const { uiMode } = (useOutletContext() as { uiMode?: 'modern' }) || {};
  const { locale } = useI18n();
  const modern = uiMode === 'modern';
  const [sessions, setSessions] = useState<PanelChatSession[]>([]);
  const [selectedId, setSelectedId] = useState('');
  const [messages, setMessages] = useState<PanelChatMessage[]>([]);
  const [agents, setAgents] = useState<AgentOption[]>([]);
  const [draftMode, setDraftMode] = useState<'direct' | 'group'>('direct');
  const [selectedAgentId, setSelectedAgentId] = useState('main');
  const [selectedAgentIds, setSelectedAgentIds] = useState<string[]>([]);
  const [summaryAgentId, setSummaryAgentId] = useState('');
  const [participants, setParticipants] = useState<SessionParticipant[]>([]);
  const [sharedContexts, setSharedContexts] = useState<SharedContextItem[]>([]);
  const [sharedContextInput, setSharedContextInput] = useState('');
  const [input, setInput] = useState('');
  const [loading, setLoading] = useState(false);
  const [processingSessionId, setProcessingSessionId] = useState('');
  const [booting, setBooting] = useState(true);
  const [creating, setCreating] = useState(false);
  const [renaming, setRenaming] = useState(false);
  const [draftTitle, setDraftTitle] = useState('');
  const [highlightedId, setHighlightedId] = useState('');
  const [pendingUserMessage, setPendingUserMessage] = useState<PanelChatMessage | null>(null);
  const [errorText, setErrorText] = useState('');
  const [copiedCode, setCopiedCode] = useState('');
  const [abortedMarkers, setAbortedMarkers] = useState<Record<string, PanelChatMessage[]>>({});
  const messageListRef = useRef<HTMLDivElement | null>(null);
  const previousAssistantRef = useRef('');
  const selectedIdRef = useRef('');
  const abortControllerRef = useRef<AbortController | null>(null);
  const abortMarkerHandledRef = useRef<Record<string, boolean>>({});
  const activeRequestIdRef = useRef(0);

  const text = useMemo(() => {
    if (locale === 'en') {
      return {
        title: 'Panel Chat',
        subtitle: 'Talk to the local OpenClaw agent directly in the panel.',
        newChat: 'New chat',
        emptyTitle: 'Start a local OpenClaw conversation',
        emptyDesc: 'Configure a model first, then chat with OpenClaw here directly.',
        hint: 'OpenClaw can use local files, workspace context, and installed skills here just like its native local mode.',
        input: 'Ask OpenClaw to analyze files, edit code, or use installed skills...',
        agentLabel: 'Agent',
        agentForNewChat: 'Agent for new chat',
        groupAgents: 'Agents in group chat',
        summaryAgent: 'Summary AI',
        minGroupAgents: 'Select at least 2 AI roles for group chat.',
        sharedContext: 'Shared files',
        sharedContextHint: 'One file path per line, using workspace-relative paths.',
        sourceLabel: 'Sources',
        createCompanyTask: 'Create task',
        companyTaskCreated: 'Company task created successfully.',
        groupMode: 'Multi-AI sequence',
        directMode: 'Single AI',
        modeSingle: 'Single chat',
        modeGroup: 'Group chat',
        newDirectChat: 'New single chat',
        newGroupChat: 'New group chat',
        participants: 'Participants',
        replyingNow: 'Replying now',
        defaultAgentSuffix: 'default',
        sending: 'Sending...',
        stop: 'Stop',
        processing: 'OpenClaw is thinking...',
        send: 'Send',
        delete: 'Delete',
        rename: 'Rename',
        renamePlaceholder: 'Session title',
        loading: 'Loading...',
        failedLoad: 'Failed to load chat data. Refresh and try again.',
        failedCreate: 'Failed to create a chat session.',
        failedDetail: 'Failed to load conversation detail.',
        failedSend: 'Send failed. Your draft has been restored.',
        failedRename: 'Rename failed.',
        failedDelete: 'Delete failed.',
        copy: 'Copy',
        copied: 'Copied',
        enterHint: 'Enter to send, Shift + Enter for newline',
        deleteConfirm: 'Delete this panel chat session?',
        noSessions: 'No panel chats yet',
        noMessages: 'No messages yet',
      };
    }
    return {
      title: '面板聊天',
      subtitle: '直接在面板里和本地 OpenClaw 交互。',
      newChat: '新建会话',
      emptyTitle: '开始一段本地 OpenClaw 对话',
      emptyDesc: '先在系统配置里配好模型，然后就可以在这里与 OpenClaw 直接聊天。',
      hint: '这里会直接调用本地 OpenClaw，能继续使用它已安装的技能、工作区上下文和本地文件能力。',
      input: '给 OpenClaw 发消息，比如让它分析文件、修改代码或调用已安装技能...',
      agentLabel: '智能体',
      agentForNewChat: '新会话使用智能体',
      groupAgents: '群聊参与 AI',
      summaryAgent: '总结 AI',
      minGroupAgents: '群聊至少选择 2 个 AI 角色。',
      sharedContext: '公共资料',
      sharedContextHint: '每行一个文件路径，建议使用工作区相对路径。',
      sourceLabel: '引用来源',
      createCompanyTask: '发起协作任务',
      companyTaskCreated: '协作任务已创建。',
      groupMode: '多 AI 顺序回复',
      directMode: '单 AI 对话',
      modeSingle: '单聊',
      modeGroup: '群聊',
      newDirectChat: '新建单聊',
      newGroupChat: '新建群聊',
      participants: '参与角色',
      replyingNow: '当前回复',
      defaultAgentSuffix: '默认',
      sending: '发送中...',
      stop: '中止',
      processing: 'OpenClaw 思考中...',
      send: '发送',
      delete: '删除',
      rename: '重命名',
      renamePlaceholder: '会话标题',
      loading: '加载中...',
      failedLoad: '聊天数据加载失败，请刷新后重试。',
      failedCreate: '创建会话失败。',
      failedDetail: '加载会话详情失败。',
      failedSend: '发送失败，已恢复你的输入内容。',
      failedRename: '重命名失败。',
      failedDelete: '删除失败。',
      copy: '复制',
      copied: '已复制',
      enterHint: 'Enter 发送，Shift + Enter 换行',
      deleteConfirm: '确定删除当前面板会话吗？',
      noSessions: '还没有面板会话',
      noMessages: '还没有消息',
    };
  }, [locale]);

  const selectedSession = sessions.find(item => item.id === selectedId) || null;
  const processing = (!!selectedId && processingSessionId === selectedId) || !!selectedSession?.processing;
  const interactionLocked = loading || !!processingSessionId || creating;
  const currentRespondingAgent = selectedSession?.currentAgentName || selectedSession?.currentAgentId || '';
  const isGroupDraft = draftMode === 'group';
  const visibleSessions = useMemo(() => sessions.filter(session => draftMode === 'group' ? session.chatType === 'group' : session.chatType !== 'group'), [draftMode, sessions]);
  const timelineMessages = useMemo(() => {
    const pending = pendingUserMessage && pendingUserMessage.sessionId === selectedId && !messages.some(item => item.role === 'user' && normalizeUserMessageContent(item.content) === normalizeUserMessageContent(pendingUserMessage.content)) ? [pendingUserMessage] : [];
    return [...messages, ...(abortedMarkers[selectedId] || []), ...pending].sort((a, b) => {
      const ta = new Date(a.timestamp).getTime();
      const tb = new Date(b.timestamp).getTime();
      if (ta === tb) return a.id.localeCompare(b.id);
      return ta - tb;
    });
  }, [abortedMarkers, messages, pendingUserMessage, selectedId]);

  useEffect(() => {
    selectedIdRef.current = selectedId;
  }, [selectedId]);

  useEffect(() => {
    setDraftTitle(selectedSession?.title || '');
  }, [selectedSession?.id, selectedSession?.title]);

  useEffect(() => {
    if (!selectedSession) setRenaming(false);
  }, [selectedSession]);

  useEffect(() => {
    if (!pendingUserMessage || pendingUserMessage.sessionId !== selectedId) return;
    const matched = messages.some(item => item.role === 'user' && normalizeUserMessageContent(item.content) === normalizeUserMessageContent(pendingUserMessage.content));
    if (matched) setPendingUserMessage(null);
  }, [messages, pendingUserMessage, selectedId]);

  const loadSessions = useCallback(async (preferredId?: string) => {
    const res = await api.getPanelChatSessions();
    if (!res?.ok) {
      setErrorText(text.failedLoad);
      return;
    }
    const next = Array.isArray(res.sessions) ? res.sessions : [];
    setErrorText('');
    setSessions(next);
    setSelectedId(current => {
      if (preferredId && next.some((item: PanelChatSession) => item.id === preferredId)) return preferredId;
      if (current && next.some((item: PanelChatSession) => item.id === current)) return current;
      return next[0]?.id || '';
    });
  }, [text.failedLoad]);

  const loadAgents = useCallback(async () => {
    try {
      const res = await api.getAgentsConfig();
      const list = Array.isArray(res?.agents?.list) ? res.agents.list : [];
      const normalized = list
        .map((item: any) => ({ id: String(item?.id || '').trim(), name: String(item?.name || '').trim(), isDefault: !!item?.default }))
        .filter((item: AgentOption) => item.id);
      setAgents(normalized);
      const preferred = normalized.find((item: AgentOption) => item.isDefault)?.id || normalized[0]?.id || 'main';
      setSelectedAgentId(current => current || preferred);
      setSelectedAgentIds(current => current.length > 0 ? current : [preferred]);
      setSummaryAgentId(current => current || preferred);
      if (!selectedAgentId) setSelectedAgentId(preferred);
    } catch {
      setAgents([{ id: 'main', isDefault: true }]);
      setSelectedAgentIds(current => current.length > 0 ? current : ['main']);
      setSummaryAgentId(current => current || 'main');
      if (!selectedAgentId) setSelectedAgentId('main');
    }
  }, [selectedAgentId]);

  const loadDetail = useCallback(async (id: string) => {
    if (!id) {
      setMessages([]);
      setParticipants([]);
      setSharedContexts([]);
      return;
    }
    const res = await api.getPanelChatSessionDetail(id);
    if (!res?.ok) {
      setErrorText(text.failedDetail);
      return;
    }
    setErrorText('');
    setMessages(Array.isArray(res.messages) ? res.messages : []);
    setParticipants(Array.isArray(res.participants) ? res.participants : []);
    const nextSharedContexts = Array.isArray(res.sharedContexts) ? res.sharedContexts : [];
    setSharedContexts(nextSharedContexts);
    setSharedContextInput(nextSharedContexts.map((item: SharedContextItem) => item.path).join('\n'));
    setPendingUserMessage(null);
  }, [text.failedDetail]);

  useEffect(() => {
    (async () => {
      try {
        await Promise.all([loadSessions(), loadAgents()]);
      } finally {
        setBooting(false);
      }
    })();
  }, [loadAgents, loadSessions]);

  useEffect(() => {
    void loadDetail(selectedId);
  }, [loadDetail, selectedId]);

  useEffect(() => {
    const container = messageListRef.current;
    if (!container) return;
    requestAnimationFrame(() => {
      container.scrollTop = container.scrollHeight;
    });
  }, [messages, pendingUserMessage, processing]);

  useEffect(() => {
    if (!selectedSession) return;
    const belongsToMode = draftMode === 'group' ? selectedSession.chatType === 'group' : selectedSession.chatType !== 'group';
    if (!belongsToMode) {
      const fallback = visibleSessions[0]?.id || '';
      setSelectedId(fallback);
      if (!fallback) {
        setMessages([]);
        setParticipants([]);
      }
    }
  }, [draftMode, selectedSession, visibleSessions]);

  const toggleDraftAgent = useCallback((agentId: string) => {
    setSelectedAgentIds(current => {
      if (current.includes(agentId)) {
        const next = current.filter(item => item !== agentId);
        return next.length > 0 ? next : [agentId];
      }
      return [...current, agentId];
    });
  }, []);

  useEffect(() => {
    const lastAssistant = [...messages].reverse().find(message => message.role === 'assistant');
    if (!lastAssistant) return;
    if (lastAssistant.id !== previousAssistantRef.current) {
      previousAssistantRef.current = lastAssistant.id;
      setHighlightedId(lastAssistant.id);
      const timer = window.setTimeout(() => setHighlightedId(current => current === lastAssistant.id ? '' : current), 3500);
      return () => window.clearTimeout(timer);
    }
  }, [messages]);

  const createSession = useCallback(async (agentId = selectedAgentId, mode: 'direct' | 'group' = 'direct') => {
    if (creating) return '';
    setCreating(true);
    setErrorText('');
    try {
      const participantIds = mode === 'group'
        ? Array.from(new Set((selectedAgentIds.length > 0 ? selectedAgentIds : [agentId]).filter(Boolean)))
        : [agentId];
      const sharedContextPaths = Array.from(new Set(sharedContextInput.split(/\r?\n/).map(item => item.trim()).filter(Boolean)));
      if (mode === 'group' && participantIds.length < 2) {
        setErrorText(text.minGroupAgents);
        return '';
      }
      const primaryAgentId = mode === 'group' ? (participantIds[0] || agentId) : agentId;
      const res = await api.createPanelChatSession({
        chatType: mode === 'group' || participantIds.length > 1 || isGroupDraft ? 'group' : 'direct',
        agentId: primaryAgentId,
        agentIds: participantIds,
        summaryAgentId: mode === 'group' ? (summaryAgentId || undefined) : undefined,
        sharedContextPaths,
      });
      if (!res?.ok || !res.session?.id) {
        setErrorText(res?.error || text.failedCreate);
        return '';
      }
      await loadSessions(res.session.id);
      setSelectedId(res.session.id);
      setMessages([]);
      setParticipants(Array.isArray(res.participants) ? res.participants : []);
      setSharedContexts(sharedContextPaths.map(path => ({ path, title: path.split('/').pop() || path })));
      return res.session.id as string;
    } finally {
      setCreating(false);
    }
  }, [creating, isGroupDraft, loadSessions, selectedAgentId, selectedAgentIds, sharedContextInput, summaryAgentId, text.failedCreate, text.minGroupAgents]);

  const appendAbortMarker = useCallback((sessionId: string) => {
    if (abortMarkerHandledRef.current[sessionId]) return;
    abortMarkerHandledRef.current[sessionId] = true;
    const marker: PanelChatMessage = {
      id: `abort-${Date.now()}`,
      role: 'system',
      content: locale === 'en' ? 'Generation stopped' : '已中止',
      timestamp: new Date().toISOString(),
      sessionId,
    };
    setAbortedMarkers(current => ({
      ...current,
      [sessionId]: [...(current[sessionId] || []), marker],
    }));
  }, [locale]);

  const handleAbort = useCallback(() => {
    const sessionId = processingSessionId || selectedIdRef.current;
    if (!sessionId) return;
    activeRequestIdRef.current += 1;
    abortControllerRef.current?.abort();
    abortControllerRef.current = null;
    void api.cancelPanelChatMessage(sessionId);
    setProcessingSessionId('');
    setLoading(false);
    setPendingUserMessage(null);
    appendAbortMarker(sessionId);
  }, [appendAbortMarker, processingSessionId]);

  const handleSend = useCallback(async () => {
    const message = input.trim();
    if (!message || loading) return;
    let sessionId = selectedId;
    const requestId = activeRequestIdRef.current + 1;
    activeRequestIdRef.current = requestId;
    setErrorText('');
    setLoading(true);
    setInput('');
    try {
      if (!sessionId) sessionId = await createSession(selectedAgentId, draftMode);
      if (!sessionId) return;
      setPendingUserMessage({
        id: `pending-user-${Date.now()}`,
        role: 'user',
        content: message,
        timestamp: new Date().toISOString(),
        sessionId,
      });
      setProcessingSessionId(sessionId);
      const controller = new AbortController();
      abortControllerRef.current = controller;
      const token = localStorage.getItem('admin-token') || '';
      const response = await fetch(`/api/panel-chat/sessions/${sessionId}/messages`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          ...(token ? { Authorization: `Bearer ${token}` } : {}),
        },
        body: JSON.stringify({ message }),
        signal: controller.signal,
      });
      const res = await response.json();
      abortControllerRef.current = null;
      if (activeRequestIdRef.current !== requestId) return;
      if (res?.ok) {
        abortMarkerHandledRef.current[sessionId] = false;
        if (selectedIdRef.current === sessionId) {
          setMessages(Array.isArray(res.messages) ? res.messages : []);
          setParticipants(Array.isArray(res.participants) ? res.participants : []);
          setPendingUserMessage(null);
        }
        await loadSessions(sessionId);
      } else if (res?.canceled) {
        appendAbortMarker(sessionId);
        await loadSessions(sessionId);
      } else {
        if (selectedIdRef.current === sessionId) setPendingUserMessage(null);
        setInput(message);
        setErrorText(res?.error || text.failedSend);
      }
    } catch (error: any) {
      abortControllerRef.current = null;
      if (activeRequestIdRef.current !== requestId) return;
      if (error?.name === 'AbortError') {
        if (sessionId) appendAbortMarker(sessionId);
        return;
      }
      setPendingUserMessage(null);
      setInput(message);
      setErrorText(text.failedSend);
    } finally {
      if (activeRequestIdRef.current !== requestId) return;
      if (sessionId) {
        if (!processingSessionId || processingSessionId === sessionId) abortMarkerHandledRef.current[sessionId] = false;
        setProcessingSessionId(current => current === sessionId ? '' : current);
      }
      setLoading(false);
    }
  }, [appendAbortMarker, createSession, draftMode, input, loadSessions, loading, processingSessionId, selectedAgentId, selectedId, text.failedSend]);

  const handleCreateCompanyTask = useCallback(async () => {
    const goal = input.trim();
    if (!goal || loading || creating) return;
    let sessionId = selectedId;
    setErrorText('');
    setLoading(true);
    try {
      if (!sessionId) sessionId = await createSession(selectedAgentId, draftMode);
      if (!sessionId) return;
      const managerAgentId = selectedSession?.summaryAgentId || selectedSession?.agentId || summaryAgentId || selectedAgentId;
      const workerAgentIds = (participants.length > 0 ? participants.map(item => item.agentId) : selectedAgentIds).filter(id => id && id !== managerAgentId);
      const res = await api.createCompanyTask({
        title: goal,
        goal,
        sourceType: 'panel_chat',
        deliveryType: 'write_back_panel_session',
        panelSessionId: sessionId,
        deliveryTargetId: sessionId,
        managerAgentId,
        summaryAgentId: managerAgentId,
        workerAgentIds,
      });
      if (!res?.ok) {
        setErrorText(res?.error || text.failedSend);
        return;
      }
      setInput('');
      setPendingUserMessage(null);
      await loadDetail(sessionId);
      await loadSessions(sessionId);
    } catch {
      setErrorText(text.failedSend);
    } finally {
      setLoading(false);
    }
  }, [createSession, creating, draftMode, input, loadDetail, loadSessions, loading, participants, selectedAgentId, selectedAgentIds, selectedId, selectedSession, summaryAgentId, text.failedSend]);

  const handleDelete = useCallback(async () => {
    if (!selectedSession || interactionLocked || !window.confirm(text.deleteConfirm)) return;
    const deletingId = selectedSession.id;
    const fallback = sessions.find(item => item.id !== deletingId)?.id || '';
    const res = await api.deletePanelChatSession(deletingId);
    if (!res?.ok) {
      setErrorText(text.failedDelete);
      return;
    }
    setErrorText('');
    setSelectedId(fallback);
    if (!fallback) setMessages([]);
    await loadSessions(fallback);
  }, [interactionLocked, loadSessions, selectedSession, sessions, text.deleteConfirm, text.failedDelete]);

  const handleRename = useCallback(async () => {
    if (!selectedSession) return;
    const title = draftTitle.trim();
    if (!title || title === selectedSession.title) {
      setRenaming(false);
      setDraftTitle(selectedSession.title);
      return;
    }
    const res = await api.renamePanelChatSession(selectedSession.id, title);
    if (!res?.ok) {
      setErrorText(text.failedRename);
      return;
    }
    setErrorText('');
    setRenaming(false);
    await loadSessions(selectedSession.id);
  }, [draftTitle, loadSessions, selectedSession, text.failedRename]);

  const handleCopyCode = useCallback(async (content: string) => {
    try {
      if (navigator.clipboard?.writeText) {
        await navigator.clipboard.writeText(content);
      } else {
        throw new Error('clipboard api unavailable');
      }
      setCopiedCode(content);
      window.setTimeout(() => {
        setCopiedCode(current => current === content ? '' : current);
      }, 1800);
    } catch {
      setErrorText(locale === 'en' ? 'Copy failed.' : '复制失败。');
    }
  }, [locale]);

  return (
    <div className={`flex h-full min-h-0 flex-col gap-4 ${modern ? 'page-modern' : ''}`}>
      <section className={modern ? 'page-modern-header shrink-0' : 'ui-modern-card flex flex-wrap items-start justify-between gap-4 p-5'}>
        <div>
          <h2 className={modern ? 'page-modern-title text-xl' : 'text-xl font-bold text-gray-900 dark:text-white'}>{text.title}</h2>
          <p className={modern ? 'page-modern-subtitle mt-1 text-sm' : 'text-sm text-gray-500 mt-1'}>{text.subtitle}</p>
        </div>
        <div className="flex items-center gap-2 rounded-2xl border border-blue-100 bg-blue-50/80 px-3 py-2 text-xs text-blue-700 dark:border-blue-500/20 dark:bg-blue-500/10 dark:text-blue-200">
          <Bot size={14} />
          <span>{text.hint}</span>
        </div>
      </section>

      {errorText && (
        <section className="ui-modern-card border border-rose-200 bg-rose-50/90 px-4 py-3 text-sm text-rose-700 dark:border-rose-500/20 dark:bg-rose-500/10 dark:text-rose-200">
          {errorText}
        </section>
      )}

      <section className={`grid min-h-0 flex-1 gap-4 ${modern ? 'xl:grid-cols-[320px_minmax(0,1fr)]' : 'lg:grid-cols-[320px_minmax(0,1fr)]'}`}>
        <aside className={`${modern ? 'page-modern-panel' : 'ui-modern-card'} flex min-h-[240px] flex-col overflow-hidden p-0`}>
          <div className="shrink-0 border-b border-slate-200/70 bg-white/80 px-4 py-4 dark:border-slate-700/70 dark:bg-slate-950/40">
            <div className="space-y-3">
              <div className="grid grid-cols-2 gap-2 rounded-2xl border border-slate-200 bg-slate-100/80 p-1 dark:border-slate-700 dark:bg-slate-900/70">
                <button
                  type="button"
                  onClick={() => setDraftMode('direct')}
                  className={`rounded-xl px-3 py-2 text-xs font-medium transition ${draftMode === 'direct' ? 'bg-white text-slate-900 shadow-sm dark:bg-slate-800 dark:text-slate-100' : 'text-slate-500 hover:text-slate-700 dark:text-slate-400 dark:hover:text-slate-200'}`}
                >
                  {text.modeSingle}
                </button>
                <button
                  type="button"
                  onClick={() => setDraftMode('group')}
                  className={`rounded-xl px-3 py-2 text-xs font-medium transition ${draftMode === 'group' ? 'bg-white text-slate-900 shadow-sm dark:bg-slate-800 dark:text-slate-100' : 'text-slate-500 hover:text-slate-700 dark:text-slate-400 dark:hover:text-slate-200'}`}
                >
                  {text.modeGroup}
                </button>
              </div>
              {draftMode === 'direct' && (
                <div>
                  <div className="mb-1 text-[11px] font-medium uppercase tracking-[0.16em] text-slate-400 dark:text-slate-500">{text.agentForNewChat}</div>
                  <select
                    value={selectedAgentId}
                    onChange={event => {
                      setSelectedAgentId(event.target.value);
                      setSelectedAgentIds([event.target.value]);
                      setSummaryAgentId(current => current || event.target.value);
                    }}
                    className="w-full rounded-xl border border-slate-200 bg-white px-3 py-2 text-sm text-slate-700 outline-none dark:border-slate-700 dark:bg-slate-900 dark:text-slate-100"
                  >
                    {agents.map(agent => (
                      <option key={agent.id} value={agent.id}>
                        {agentDisplayName(agent)}{agent.isDefault ? ` · ${text.defaultAgentSuffix}` : ''}
                      </option>
                    ))}
                  </select>
                </div>
              )}
              {draftMode === 'group' && (
                <>
                  <div>
                    <div className="mb-1 text-[11px] font-medium uppercase tracking-[0.16em] text-slate-400 dark:text-slate-500">{text.groupAgents}</div>
                    <div className="flex flex-wrap gap-2">
                      {agents.map(agent => {
                        const active = selectedAgentIds.includes(agent.id);
                        return (
                          <button
                            key={`draft-${agent.id}`}
                            type="button"
                            onClick={() => toggleDraftAgent(agent.id)}
                            className={`rounded-full px-3 py-1.5 text-xs transition ${active ? agentBadgeTone(agent.id) : 'border border-slate-200 bg-white text-slate-500 dark:border-slate-700 dark:bg-slate-900 dark:text-slate-400'}`}
                          >
                            {agentDisplayName(agent)}
                          </button>
                        );
                      })}
                    </div>
                  </div>
                  <div>
                    <div className="mb-1 text-[11px] font-medium uppercase tracking-[0.16em] text-slate-400 dark:text-slate-500">{text.summaryAgent}</div>
                    <select
                      value={summaryAgentId}
                      onChange={event => setSummaryAgentId(event.target.value)}
                      className="w-full rounded-xl border border-slate-200 bg-white px-3 py-2 text-sm text-slate-700 outline-none dark:border-slate-700 dark:bg-slate-900 dark:text-slate-100"
                    >
                      {agents.map(agent => (
                        <option key={`summary-${agent.id}`} value={agent.id}>{agentDisplayName(agent)}</option>
                      ))}
                    </select>
                  </div>
                  <div>
                    <div className="mb-1 text-[11px] font-medium uppercase tracking-[0.16em] text-slate-400 dark:text-slate-500">{text.sharedContext}</div>
                    <textarea
                      value={sharedContextInput}
                      onChange={event => setSharedContextInput(event.target.value)}
                      rows={4}
                      placeholder={text.sharedContextHint}
                      className="w-full rounded-xl border border-slate-200 bg-white px-3 py-2 text-sm text-slate-700 outline-none placeholder:text-slate-400 dark:border-slate-700 dark:bg-slate-900 dark:text-slate-100"
                    />
                  </div>
                </>
              )}
              <button onClick={() => void createSession(selectedAgentId, draftMode)} disabled={interactionLocked} className="inline-flex w-full items-center justify-center gap-1.5 rounded-xl bg-slate-900 px-3 py-2 text-xs font-medium text-white transition hover:bg-slate-800 disabled:cursor-not-allowed disabled:opacity-60 dark:bg-white dark:text-slate-900 dark:hover:bg-slate-100">
                {creating ? <Loader2 size={14} className="animate-spin" /> : <MessageSquarePlus size={14} />}
                {draftMode === 'group' ? text.newGroupChat : text.newDirectChat}
              </button>
            </div>
            <div className="mt-3 flex items-center gap-2 text-xs text-slate-500 dark:text-slate-400">
              <span className="font-semibold text-slate-900 dark:text-slate-100 tabular-nums">{visibleSessions.length}</span>
              <span>{draftMode === 'group' ? text.modeGroup : text.modeSingle}</span>
              {selectedSession && (
                <span className={`inline-flex items-center rounded-full p-1.5 ${sessionBadgeTone(selectedSession.chatType)}`}>
                  {selectedSession.chatType === 'group' ? <Users size={12} /> : <User size={12} />}
                </span>
              )}
            </div>
          </div>
          <div className="min-h-0 flex-1 overflow-y-auto bg-slate-50/70 p-3 dark:bg-slate-950/30">
            {booting ? (
              <div className="flex h-full items-center justify-center text-sm text-slate-400"><Loader2 size={16} className="mr-2 animate-spin" />{text.loading}</div>
            ) : visibleSessions.length === 0 ? (
              <div className="flex h-full items-center justify-center px-4 text-center text-sm text-slate-400">{text.noSessions}</div>
            ) : (
              <div className="space-y-2">
                {visibleSessions.map(session => (
                  <button
                    key={session.id}
                    onClick={() => {
                      if (creating) return;
                      setErrorText('');
                      setSelectedId(session.id);
                    }}
                    disabled={creating}
                    className={`w-full rounded-2xl border px-3 py-3 text-left transition ${selectedId === session.id ? 'border-blue-200 bg-blue-50 shadow-sm ring-1 ring-blue-100 dark:border-blue-500/40 dark:bg-blue-500/12 dark:ring-blue-500/20' : 'border-transparent bg-white/75 hover:border-slate-200 hover:bg-white dark:bg-slate-900/30 dark:hover:border-slate-800 dark:hover:bg-slate-900/55'}`}
                  >
                    <div className="flex items-center justify-between gap-2">
                      <span className={`truncate text-sm font-semibold ${selectedId === session.id ? 'text-blue-900 dark:text-blue-100' : 'text-slate-800 dark:text-slate-100'}`}>{session.title || text.newChat}</span>
                      <span className={`inline-flex items-center rounded-full p-1 text-[10px] ${sessionBadgeTone(session.chatType)}`}>
                        {session.chatType === 'group' ? <Users size={11} /> : <User size={11} />}
                      </span>
                    </div>
                    <p className={`mt-1 line-clamp-2 text-xs ${selectedId === session.id ? 'text-blue-700 dark:text-blue-200' : 'text-slate-500 dark:text-slate-400'}`}>{session.processing ? `${text.processing}${session.currentAgentName ? ` · ${session.currentAgentName}` : session.currentAgentId ? ` · ${session.currentAgentId}` : ''}` : (session.lastMessage || `${text.agentLabel}: ${session.agentId}`)}</p>
                  </button>
                ))}
              </div>
            )}
          </div>
        </aside>

        <div className={`${modern ? 'page-modern-panel' : 'ui-modern-card'} flex min-h-[480px] min-w-0 flex-col overflow-hidden p-0`}>
          <div className="flex items-center justify-between border-b border-slate-200/70 bg-white/80 px-5 py-4 dark:border-slate-700/70 dark:bg-slate-950/40">
            <div>
              {renaming && selectedSession ? (
                <div className="flex items-center gap-2">
                  <input value={draftTitle} onChange={event => setDraftTitle(event.target.value)} onKeyDown={event => {
                    if (event.key === 'Enter') void handleRename();
                    if (event.key === 'Escape') { setRenaming(false); setDraftTitle(selectedSession.title); }
                  }} placeholder={text.renamePlaceholder} className="rounded-xl border border-slate-200 bg-white px-3 py-2 text-sm outline-none dark:border-slate-700 dark:bg-slate-900 dark:text-slate-100" />
                  <button onClick={() => void handleRename()} className="rounded-xl bg-slate-900 px-3 py-2 text-xs font-medium text-white dark:bg-white dark:text-slate-900">OK</button>
                </div>
              ) : (
                <h3 className="text-base font-semibold text-slate-900 dark:text-slate-50">{selectedSession?.title || text.emptyTitle}</h3>
              )}
              <p className="mt-1 text-xs text-slate-500 dark:text-slate-400">{selectedSession ? `${selectedSession.chatType === 'group' ? text.groupMode : text.agentLabel}: ${selectedSession.chatType === 'group' ? `${participants.length || selectedSession.participantCount || 0} AI` : (agentDisplayName(agents.find(item => item.id === selectedSession.agentId)) || selectedSession.agentId)}` : text.emptyDesc}</p>
              {selectedSession && participants.length > 0 && (
                <div className="mt-2 flex flex-wrap gap-2">
                  {participants.map(participant => (
                    <span key={`participant-${participant.agentId}`} className={`rounded-full px-2 py-1 text-[11px] ${agentBadgeTone(participant.agentId)}`}>
                      {participant.name || participant.agentId}{participant.isSummary ? ' · Summary' : ''}
                    </span>
                  ))}
                </div>
              )}
              {selectedSession && sharedContexts.length > 0 && (
                <div className="mt-2 flex flex-wrap gap-2">
                  {sharedContexts.map(item => (
                    <span key={`shared-${item.path}`} className="rounded-full border border-slate-200 bg-slate-100 px-2 py-1 text-[11px] text-slate-600 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-300">
                      {item.title || item.path}
                    </span>
                  ))}
                </div>
              )}
              {processing && <p className="mt-2 text-xs font-medium text-blue-600 dark:text-blue-300">{text.replyingNow}: {currentRespondingAgent || text.processing}</p>}
            </div>
            <div className="flex items-center gap-2">
              <button onClick={() => setRenaming(value => !value)} disabled={!selectedSession || interactionLocked} className="inline-flex items-center gap-1.5 rounded-xl border border-slate-200 px-3 py-2 text-xs text-slate-600 transition hover:bg-slate-50 disabled:cursor-not-allowed disabled:opacity-40 dark:border-slate-700 dark:text-slate-300 dark:hover:bg-slate-900">{text.rename}</button>
              <button onClick={handleDelete} disabled={!selectedSession || interactionLocked} className="inline-flex items-center gap-1.5 rounded-xl border border-slate-200 px-3 py-2 text-xs text-slate-600 transition hover:bg-slate-50 disabled:cursor-not-allowed disabled:opacity-40 dark:border-slate-700 dark:text-slate-300 dark:hover:bg-slate-900">
                <Trash2 size={14} />
                {text.delete}
              </button>
            </div>
          </div>

          <div ref={messageListRef} className="min-h-0 flex-1 overflow-y-auto bg-[radial-gradient(circle_at_top,rgba(59,130,246,0.08),transparent_24%),linear-gradient(180deg,rgba(255,255,255,0.16),transparent_36%)] px-5 py-5 dark:bg-[radial-gradient(circle_at_top,rgba(59,130,246,0.14),transparent_28%),linear-gradient(180deg,rgba(255,255,255,0.03),transparent_36%)]">
            {timelineMessages.length === 0 ? (
              <div className="flex h-full flex-col items-center justify-center gap-3 text-center text-slate-400">
                <div className="flex h-16 w-16 items-center justify-center rounded-full border border-blue-100 bg-white/80 text-blue-500 shadow-sm dark:border-blue-500/20 dark:bg-slate-900/70 dark:text-blue-200">
                  <Bot size={28} />
                </div>
                <div>
                  <p className="text-sm font-medium text-slate-500 dark:text-slate-300">{selectedSession ? text.noMessages : text.emptyTitle}</p>
                  <p className="mt-1 text-xs">{text.emptyDesc}</p>
                </div>
              </div>
            ) : (
              <div className="space-y-4">
                {timelineMessages.map(message => {
                  const isUser = message.role === 'user';
                  const isSystem = message.role === 'system';
                  const messageAgentId = message.agentId || selectedSession?.agentId || 'assistant';
                  return (
                    <Fragment key={message.id}>
                      <div className={`flex gap-3 ${isSystem ? 'justify-center' : isUser ? 'justify-end' : 'justify-start'}`}>
                        {isSystem ? (
                          <div className="my-2 flex w-full items-center gap-3 text-xs text-slate-400 dark:text-slate-500">
                            <div className="h-px flex-1 bg-slate-200 dark:bg-slate-700" />
                            <span className="shrink-0 rounded-full border border-slate-200 bg-white px-3 py-1 font-medium dark:border-slate-700 dark:bg-slate-900">{message.content}</span>
                            <div className="h-px flex-1 bg-slate-200 dark:bg-slate-700" />
                          </div>
                        ) : (
                          <>
                            {!isUser && <div className={`mt-1 flex h-8 w-8 shrink-0 items-center justify-center rounded-full ${agentBadgeTone(messageAgentId)}`}><Bot size={15} /></div>}
                            <div className={`max-w-[85%] rounded-2xl px-4 py-3 text-sm leading-6 shadow-sm transition ${isUser ? 'rounded-tr-sm bg-[linear-gradient(135deg,#1d4ed8,#0284c7)] text-right text-white shadow-blue-200/50 dark:shadow-none' : 'rounded-tl-sm border border-slate-200/70 bg-white/95 text-slate-700 dark:border-slate-700/70 dark:bg-slate-900/92 dark:text-slate-200'} ${highlightedId === message.id ? 'ring-2 ring-blue-300 dark:ring-blue-500/50' : ''}`}>
                              {!isUser && (
                                <div className="mb-2 flex items-center gap-2 text-[11px] font-medium text-slate-500 dark:text-slate-400">
                                  <span className={`rounded-full px-2 py-0.5 ${agentBadgeTone(messageAgentId)}`}>{message.agentName || message.agentId || selectedSession?.agentId || 'AI'}</span>
                                  {message.messageType === 'summary' && <span className="rounded-full border border-amber-200 bg-amber-50 px-2 py-0.5 text-amber-700 dark:border-amber-500/30 dark:bg-amber-500/10 dark:text-amber-200">Summary</span>}
                                </div>
                              )}
                              {isUser ? message.content : (
                                <>
                                  {message.content && (
                                    <ReactMarkdown
                                      remarkPlugins={[remarkGfm]}
                                      components={{
                                        p: ({ children }) => <p className="mb-2 last:mb-0">{children}</p>,
                                        ul: ({ children }) => <ul className="mb-2 list-disc space-y-1 pl-5 last:mb-0">{children}</ul>,
                                        ol: ({ children }) => <ol className="mb-2 list-decimal space-y-1 pl-5 last:mb-0">{children}</ol>,
                                        code: ({ className, children, ...props }: any) => {
                                          const isBlock = Boolean(className);
                                          return isBlock
                                            ? <code className="block text-[13px] text-slate-100">{children}</code>
                                            : <code className="rounded-md border border-slate-200 bg-slate-100 px-1.5 py-0.5 text-[13px] text-slate-800 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100" {...props}>{children}</code>;
                                        },
                                        pre: ({ children }) => {
                                          const raw = String((children as any)?.props?.children ?? '').replace(/\n$/, '');
                                          const copied = copiedCode === raw;
                                          return (
                                            <div className="my-3 overflow-hidden rounded-xl border border-slate-200 bg-slate-900 shadow-sm dark:border-slate-700">
                                              <div className="flex items-center justify-between border-b border-slate-700/80 bg-slate-800/95 px-3 py-2 text-xs text-slate-300">
                                                <span>shell</span>
                                                <button
                                                  type="button"
                                                  onClick={event => {
                                                    event.preventDefault();
                                                    event.stopPropagation();
                                                    void handleCopyCode(raw);
                                                  }}
                                                  className="inline-flex items-center gap-1 rounded-md border border-slate-600 px-2 py-1 text-slate-200 transition hover:bg-slate-700"
                                                >
                                                  {copied ? <Check size={12} /> : <Copy size={12} />}
                                                  {copied ? text.copied : text.copy}
                                                </button>
                                              </div>
                                              <pre className="overflow-x-auto px-3 py-3 text-[13px] leading-6 text-slate-100">{children}</pre>
                                            </div>
                                          );
                                        },
                                      }}
                                    >
                                      {message.content}
                                    </ReactMarkdown>
                                  )}
                                  {Array.isArray(message.images) && message.images.length > 0 && (
                                    <div className="mt-3 space-y-3">
                                      {message.images.map((image, index) => (
                                        <a key={`${message.id}-image-${index}`} href={image.src} target="_blank" rel="noreferrer" className="block overflow-hidden rounded-2xl border border-slate-200 bg-white shadow-sm transition hover:shadow-md dark:border-slate-700 dark:bg-slate-950">
                                          <img src={image.src} alt={`assistant-image-${index + 1}`} className="max-h-[420px] w-full object-contain bg-slate-50 dark:bg-slate-950" loading="lazy" />
                                        </a>
                                      ))}
                                    </div>
                                  )}
                                  {Array.isArray(message.sources) && message.sources.length > 0 && (
                                    <div className="mt-3 rounded-xl border border-slate-200/80 bg-slate-50/80 px-3 py-2 text-xs text-slate-600 dark:border-slate-700 dark:bg-slate-800/70 dark:text-slate-300">
                                      <div className="mb-1 font-medium text-slate-500 dark:text-slate-400">{text.sourceLabel}</div>
                                      <div className="space-y-1">
                                        {message.sources.map((source, index) => (
                                          <div key={`${message.id}-source-${index}`} className="truncate">
                                            {source.title || source.path}
                                          </div>
                                        ))}
                                      </div>
                                    </div>
                                  )}
                                </>
                              )}
                              <div className={`mt-2 text-[11px] ${isUser ? 'text-white/75' : 'text-slate-400 dark:text-slate-500'}`}>{new Date(message.timestamp).toLocaleString()}</div>
                            </div>
                          </>
                        )}
                      </div>
                    </Fragment>
                  );
                })}
                {processing && (
                  <div className="flex gap-3 justify-start">
                    <div className="mt-1 flex h-8 w-8 shrink-0 items-center justify-center rounded-full border border-slate-200 bg-white text-slate-500 dark:border-slate-700 dark:bg-slate-900 dark:text-slate-300"><Bot size={15} /></div>
                    <div className="max-w-[85%] rounded-2xl rounded-tl-sm border border-dashed border-blue-200 bg-white/90 px-4 py-3 text-sm leading-6 text-slate-700 shadow-sm animate-pulse dark:border-blue-500/30 dark:bg-slate-900/90 dark:text-slate-200">
                      <div className="mb-2 text-[11px] font-medium text-slate-500 dark:text-slate-400">{text.replyingNow}: {currentRespondingAgent || (selectedSession?.chatType === 'group' ? text.groupMode : (selectedSession?.agentId || 'AI'))}</div>
                      <div className="flex items-center gap-1.5 text-blue-500 dark:text-blue-300">
                        <span className="h-2 w-2 rounded-full bg-current animate-bounce [animation-delay:-0.3s]" />
                        <span className="h-2 w-2 rounded-full bg-current animate-bounce [animation-delay:-0.15s]" />
                        <span className="h-2 w-2 rounded-full bg-current animate-bounce" />
                      </div>
                    </div>
                  </div>
                )}
              </div>
            )}
          </div>

          <div className="border-t border-slate-200/70 bg-white/85 px-5 py-4 dark:border-slate-700/70 dark:bg-slate-950/60">
            <div className="rounded-3xl border border-slate-200 bg-white p-2 shadow-sm dark:border-slate-700 dark:bg-slate-950/90">
              <textarea
                value={input}
                onChange={event => setInput(event.target.value)}
                onKeyDown={event => {
                  if (event.key === 'Enter' && !event.shiftKey) {
                    event.preventDefault();
                    void handleSend();
                  }
                }}
                rows={3}
                placeholder={text.input}
                className="w-full resize-none bg-transparent px-3 py-2 text-sm outline-none placeholder:text-slate-400 dark:text-slate-100"
              />
              <div className="flex items-center justify-between px-2 pb-1 pt-2">
                <div className="text-xs text-slate-400">{text.enterHint}</div>
                <div className="flex items-center gap-2">
                  <button onClick={() => void handleCreateCompanyTask()} disabled={loading || creating || !input.trim()} className="inline-flex items-center gap-2 rounded-2xl border border-slate-200 px-4 py-2 text-sm font-medium text-slate-700 transition disabled:cursor-not-allowed disabled:opacity-50 hover:bg-slate-50 dark:border-slate-700 dark:text-slate-200 dark:hover:bg-slate-800">
                    <Bot size={15} />
                    {text.createCompanyTask}
                  </button>
                  <button onClick={loading ? handleAbort : () => void handleSend()} disabled={creating || (!loading && !input.trim())} className={`inline-flex items-center gap-2 rounded-2xl px-4 py-2 text-sm font-medium text-white transition disabled:cursor-not-allowed disabled:opacity-50 ${loading ? 'bg-rose-600 hover:bg-rose-500 dark:bg-rose-500 dark:text-white dark:hover:bg-rose-400' : 'bg-slate-900 hover:bg-slate-800 dark:bg-white dark:text-slate-900 dark:hover:bg-slate-100'}`}>
                    {loading ? <Square size={15} fill="currentColor" /> : <Send size={16} />}
                    {loading ? text.stop : text.send}
                  </button>
                </div>
              </div>
            </div>
          </div>
        </div>
      </section>
    </div>
  );
}
