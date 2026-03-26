import { Fragment, useCallback, useEffect, useLayoutEffect, useMemo, useRef, useState } from 'react';
import { useOutletContext } from 'react-router-dom';
import { Bot, Check, ChevronDown, ChevronUp, Copy, Loader2, MessageSquarePlus, Send, Square, Trash2, User, Users } from 'lucide-react';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import { api } from '../lib/api';
import { useI18n } from '../i18n';

type PanelChatSession = {
  id: string;
  openclawSessionId: string;
  agentId: string;
  chatType: 'direct' | 'group';
  participantAgentIds?: string[];
  controllerAgentId?: string;
  preferredAgentId?: string;
  groupAgentSessionIds?: Record<string, string>;
  status?: 'idle' | 'dispatching' | 'running' | 'reviewing' | 'done';
  title: string;
  targetId?: string;
  targetName?: string;
  createdAt: number;
  updatedAt: number;
  processing?: boolean;
  messageCount: number;
  lastMessage?: string;
};

type PanelChatMessage = {
  id: string;
  role: string;
  content: string;
  timestamp: string;
  sessionId?: string;
  images?: { src: string; mimeType?: string }[];
  agentId?: string;
  stage?: 'user' | 'plan' | 'dispatch' | 'report' | 'review' | 'final';
  internal?: boolean;
};

type GroupSessionDraft = {
  open: boolean;
  title: string;
  agentIds: string[];
};

type ChatMode = 'direct' | 'group';

type GroupMessageView = 'all' | 'internal' | 'final';

type AgentOption = {
  id: string;
  name?: string;
  isDefault?: boolean;
};

type GroupTaskBundle = {
  taskId: string;
  meta: {
    status: string;
    currentStage: string;
    title: string;
  };
  spec?: string;
  result?: string;
  error?: string;
  timeline?: Array<{ time: string; type: string; agentId?: string; targetAgentId?: string; message?: string }>;
  subtasks?: Record<string, { agentId: string; role: string; status: string; summary?: string }>;
  artifacts?: Record<string, string>;
};

function normalizeUserMessageContent(content: string) {
  return content.replace(/^\[[^\]]+\]\s*/, '').trim();
}

function stageLabel(stage: PanelChatMessage['stage'], locale: string) {
  if (locale === 'en') {
    switch (stage) {
      case 'user': return 'User Request';
      case 'plan': return 'Main Agent Analysis';
      case 'dispatch': return 'Task Dispatch';
      case 'report': return 'Agent Reports';
      case 'review': return 'Reviewer Validation';
      case 'final': return 'Main Agent Summary';
      default: return '';
    }
  }
  switch (stage) {
    case 'user': return '用户任务';
    case 'plan': return '主 Agent 分析';
    case 'dispatch': return '任务分派';
    case 'report': return 'Agent 回报';
    case 'review': return '校验复核';
    case 'final': return '主 Agent 汇总';
    default: return '';
  }
}

function isDiagramLine(line: string) {
  const trimmed = line.trim();
  if (!trimmed) return false;
  if (/^#{1,6}\s/.test(trimmed)) return false;
  if (/^[*-]\s/.test(trimmed)) return false;
  if (/[┌┐└┘├┤┬┴┼│─]/.test(trimmed)) return true;
  return false;
}

function formatDiagramBlocks(content: string) {
  const lines = content.split('\n');
  const out: string[] = [];
  let buffer: string[] = [];

  const flush = () => {
    if (buffer.length === 0) return;
    if (buffer.length >= 2) {
      out.push('```text');
      out.push(...buffer);
      out.push('```');
    } else {
      out.push(...buffer);
    }
    buffer = [];
  };

  for (const line of lines) {
    if (isDiagramLine(line)) {
      buffer.push(line);
      continue;
    }
    flush();
    out.push(line);
  }
  flush();
  return out.join('\n');
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

function statusLabel(status: PanelChatSession['status'], locale: string) {
  if (locale === 'en') {
    switch (status) {
      case 'dispatching': return 'Dispatching';
      case 'running': return 'Running';
      case 'reviewing': return 'Reviewing';
      case 'done': return 'Done';
      default: return 'Idle';
    }
  }
  switch (status) {
    case 'dispatching': return '分派中';
    case 'running': return '执行中';
    case 'reviewing': return '校验中';
    case 'done': return '已完成';
    default: return '空闲';
  }
}

function groupPhaseSteps(locale: string) {
  return locale === 'en'
    ? [
        { id: 'user', label: 'Task' },
        { id: 'plan', label: 'Dispatch' },
        { id: 'report', label: 'Execute' },
        { id: 'review', label: 'Review' },
        { id: 'final', label: 'Deliver' },
      ]
    : [
        { id: 'user', label: '任务' },
        { id: 'plan', label: '分派' },
        { id: 'report', label: '执行' },
        { id: 'review', label: '校验' },
        { id: 'final', label: '交付' },
      ];
}

export default function PanelChat() {
  const { uiMode } = (useOutletContext() as { uiMode?: 'modern' }) || {};
  const { locale } = useI18n();
  const modern = uiMode === 'modern';
  const [sessions, setSessions] = useState<PanelChatSession[]>([]);
  const [chatMode, setChatMode] = useState<ChatMode>('direct');
  const [selectedId, setSelectedId] = useState('');
  const [messages, setMessages] = useState<PanelChatMessage[]>([]);
  const [groupSessions, setGroupSessions] = useState<PanelChatSession[]>([]);
  const [groupMessages, setGroupMessages] = useState<Record<string, PanelChatMessage[]>>({});
  const [agents, setAgents] = useState<AgentOption[]>([]);
  const [selectedAgentId, setSelectedAgentId] = useState('main');
  const [selectedGroupAgentId, setSelectedGroupAgentId] = useState('main');
  const [groupDraft, setGroupDraft] = useState<GroupSessionDraft>({ open: false, title: '', agentIds: ['main'] });
  const [groupTask, setGroupTask] = useState<GroupTaskBundle | null>(null);
  const [showGroupTaskCard, setShowGroupTaskCard] = useState(true);
  const [groupMessageView, setGroupMessageView] = useState<GroupMessageView>('all');
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
  const detailRequestIdRef = useRef(0);
  const abortControllerRef = useRef<AbortController | null>(null);
  const abortMarkerHandledRef = useRef<Record<string, boolean>>({});
  const activeRequestIdRef = useRef(0);

  const text = useMemo(() => {
    if (locale === 'en') {
      return {
        title: 'Panel Chat',
        subtitle: 'Talk to local OpenClaw directly in the panel, with single-agent and multi-agent collaboration modes.',
        direct: 'Direct chat',
        group: 'Group chat',
        newChat: 'New chat',
        emptyTitle: 'Start a local OpenClaw conversation',
        emptyDesc: 'Configure a model first, then chat with OpenClaw here directly.',
        hint: 'OpenClaw can use local files, workspace context, and installed skills here just like its native local mode.',
        input: 'Ask OpenClaw to analyze files, edit code, or use installed skills...',
        groupInput: 'Message the group. Main agent will coordinate the final answer.',
        agentLabel: 'Agent',
        agentForNewChat: 'Agent for new chat',
        defaultAgentSuffix: 'default',
        openWithAgent: 'New session with this agent',
        groupReplyAgent: 'Preferred group agent',
        createGroupTitle: 'Create group session',
        groupParticipantHint: 'Select agents that can join this collaboration session.',
        createGroupConfirm: 'Create group chat',
        taskCard: 'Task card',
        showTaskCard: 'Show task card',
        hideTaskCard: 'Hide task card',
        viewAll: 'All',
        viewInternal: 'Internal',
        viewFinal: 'Final',
        sending: 'Sending...',
        stop: 'Stop',
        processing: 'OpenClaw is thinking...',
        processingHint: '',
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
      subtitle: '直接在面板里和本地 OpenClaw 交互，并支持多智能体协作。',
      direct: '单聊',
      group: '群聊',
      newChat: '新建会话',
      emptyTitle: '开始一段本地 OpenClaw 对话',
      emptyDesc: '先在系统配置里配好模型，然后就可以在这里与OpenClaw直接聊天。',
      hint: '这里会直接调用本地 OpenClaw，能继续使用它已安装的技能、工作区上下文和本地文件能力。',
      input: '给 OpenClaw 发消息，比如让它分析文件、修改代码或调用已安装技能...',
      groupInput: '输入群聊任务，最终由主智能体统一汇总回复。',
      agentLabel: '智能体',
      agentForNewChat: '新会话使用智能体',
      defaultAgentSuffix: '默认',
      openWithAgent: '以该智能体另开新会话',
      groupReplyAgent: '优先处理智能体',
      createGroupTitle: '新建群聊会话',
      groupParticipantHint: '选择要参与本次协作的智能体，main 将默认作为主控。',
      createGroupConfirm: '创建群聊',
      taskCard: '协作任务卡片',
      showTaskCard: '显示任务卡片',
      hideTaskCard: '隐藏任务卡片',
      viewAll: '全部',
      viewInternal: '内部协作',
      viewFinal: '最终结论',
      sending: '发送中...',
      stop: '中止',
      processing: 'OpenClaw 思考中...',
      processingHint: '',
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

  const directSessions = useMemo(() => {
    if (!selectedAgentId) return sessions.filter(item => item.chatType === 'direct');
    return sessions.filter(item => item.chatType === 'direct' && item.agentId === selectedAgentId);
  }, [selectedAgentId, sessions]);
  const displayedSessions = directSessions;
  const selectedSession = displayedSessions.find(item => item.id === selectedId) || null;
  const liveMessages = messages;
  const processing = (!!selectedId && processingSessionId === selectedId) || !!selectedSession?.processing;
  const interactionLocked = loading || !!processingSessionId || creating;
  const sessionSwitchLocked = creating;
  const selectedAgentMeta = useMemo(() => agents.find(item => item.id === selectedSession?.agentId) || null, [agents, selectedSession?.agentId]);
  const selectedDraftAgentMeta = useMemo(() => agents.find(item => item.id === selectedAgentId) || null, [agents, selectedAgentId]);
  const timelineMessages = useMemo(() => {
    const pending = pendingUserMessage && pendingUserMessage.sessionId === selectedId && !liveMessages.some(item => item.role === 'user' && normalizeUserMessageContent(item.content) === normalizeUserMessageContent(pendingUserMessage.content)) ? [pendingUserMessage] : [];
    return [...liveMessages, ...(abortedMarkers[selectedId] || []), ...pending].sort((a, b) => {
      const ta = new Date(a.timestamp).getTime();
      const tb = new Date(b.timestamp).getTime();
      if (ta === tb) {
        const order = (role?: string) => {
          switch (role) {
            case 'user': return 0;
            case 'assistant': return 1;
            case 'system': return 2;
            default: return 3;
          }
        };
        const diff = order(a.role) - order(b.role);
        if (diff !== 0) return diff;
        return a.id.localeCompare(b.id);
      }
      return ta - tb;
    });
  }, [abortedMarkers, liveMessages, pendingUserMessage, selectedId]);

  const filteredTimelineMessages = useMemo(() => {
    if (chatMode !== 'group') return timelineMessages;
    switch (groupMessageView) {
      case 'internal':
        return timelineMessages.filter(item => item.internal || item.role === 'system');
      case 'final':
        return timelineMessages.filter(item => item.role === 'user' || (!item.internal && item.stage === 'final'));
      default:
        return timelineMessages;
    }
  }, [chatMode, groupMessageView, timelineMessages]);

  const latestGroupUserTask = useMemo(() => {
    if (chatMode !== 'group') return '';
    const latestUser = [...timelineMessages].reverse().find(item => item.role === 'user');
    return latestUser?.content || '';
  }, [chatMode, timelineMessages]);

  const groupProgress = useMemo(() => {
    const steps = groupPhaseSteps(locale);
    if (chatMode !== 'group') return { steps, activeIndex: 0 };
    if (groupTask?.subtasks) {
      const subtasks = Object.values(groupTask.subtasks);
      const hasReturned = subtasks.some(item => item.status === 'returned');
      const hasReview = subtasks.some(item => item.role === 'reviewer' && item.status === 'done');
      const allDone = subtasks.length > 0 && subtasks.every(item => item.status === 'done');
      let current = 'user';
      if (hasReturned) current = 'review';
      else if (allDone) current = 'final';
      else if (hasReview) current = 'review';
      else if (subtasks.some(item => item.status === 'done')) current = 'report';
      else if (processing) current = 'plan';
      const activeIndex = Math.max(0, steps.findIndex(step => step.id === current));
      return { steps, activeIndex };
    }
    const latestStage = [...timelineMessages].reverse().find(item => !!item.stage)?.stage || 'user';
    let current = latestStage;
    if (processing && latestStage === 'plan') current = 'report';
    const activeIndex = Math.max(0, steps.findIndex(step => step.id === current));
    return { steps, activeIndex };
  }, [chatMode, groupTask, locale, processing, timelineMessages]);

  useEffect(() => {
    selectedIdRef.current = selectedId;
  }, [selectedId]);

  useEffect(() => {
    setDraftTitle(selectedSession?.title || '');
  }, [selectedSession?.id, selectedSession?.title]);

  useEffect(() => {
    if (!selectedSession) {
      setRenaming(false);
    }
  }, [selectedSession]);

  useEffect(() => {
    if (chatMode !== 'direct') return;
    if (selectedSession?.agentId) {
      setSelectedAgentId(selectedSession.agentId);
    }
  }, [chatMode, selectedSession?.agentId]);

  useEffect(() => {
    if (chatMode !== 'group') return;
    if (!selectedSession) return;
    setSelectedGroupAgentId(selectedSession.preferredAgentId || selectedSession.controllerAgentId || selectedSession.participantAgentIds?.[0] || 'main');
  }, [chatMode, selectedSession]);

  useEffect(() => {
    if (chatMode !== 'direct') return;
    if (!selectedAgentId) return;
    if (selectedId && directSessions.some(item => item.id === selectedId)) return;
    setSelectedId(directSessions[0]?.id || '');
  }, [chatMode, directSessions, selectedAgentId, selectedId]);

  useEffect(() => {
    if (chatMode !== 'group') return;
    if (selectedId && groupSessions.some(item => item.id === selectedId)) return;
    setSelectedId(groupSessions[0]?.id || '');
  }, [chatMode, groupSessions, selectedId]);

  useEffect(() => {
    if (!pendingUserMessage || pendingUserMessage.sessionId !== selectedId) return;
    const matched = liveMessages.some(item => item.role === 'user' && normalizeUserMessageContent(item.content) === normalizeUserMessageContent(pendingUserMessage.content));
    if (matched) {
      setPendingUserMessage(null);
    }
  }, [liveMessages, pendingUserMessage, selectedId]);

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
      const defaultAgent = typeof res?.agents?.default === 'string' ? String(res.agents.default).trim() : '';
      const normalized = list.map((item: any) => ({ id: String(item?.id || '').trim(), name: String(item?.name || '').trim(), isDefault: String(item?.id || '').trim() === defaultAgent || !!item?.default })).filter((item: AgentOption) => item.id);
      setAgents(normalized);
      setSelectedAgentId(current => current || defaultAgent || normalized[0]?.id || 'main');
      setSelectedGroupAgentId(current => current || defaultAgent || normalized[0]?.id || 'main');
      setGroupDraft(current => ({ ...current, agentIds: current.agentIds.length > 0 ? current.agentIds : [defaultAgent || normalized[0]?.id || 'main'] }));
    } catch {
      const fallback = [{ id: 'main' }, { id: 'planner' }, { id: 'coder' }, { id: 'reviewer' }];
      setAgents(fallback);
      setSelectedAgentId(current => current || fallback[0].id);
      setSelectedGroupAgentId(current => current || fallback[0].id);
      setGroupDraft(current => ({ ...current, agentIds: current.agentIds.length > 0 ? current.agentIds : [fallback[0].id] }));
    }
  }, []);

  const loadDetail = useCallback(async (id: string) => {
    const requestId = detailRequestIdRef.current + 1;
    detailRequestIdRef.current = requestId;
    if (!id) {
      setMessages([]);
      setPendingUserMessage(null);
      return;
    }
    setMessages([]);
    setPendingUserMessage(null);
    const res = await api.getPanelChatSessionDetail(id);
    if (detailRequestIdRef.current != requestId) return;
    if (!res?.ok) {
      setErrorText(text.failedDetail);
      return;
    }
    setErrorText('');
    setMessages(Array.isArray(res.messages) ? res.messages : []);
    setPendingUserMessage(null);
  }, [text.failedDetail]);

  const loadLatestTask = useCallback(async (id: string) => {
    if (!id) {
      setGroupTask(null);
      return;
    }
    const res = await api.getPanelChatLatestTask(id);
    if (!res?.ok) return;
    setGroupTask(res.task || null);
  }, []);

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
    loadDetail(selectedId);
  }, [loadDetail, selectedId]);

  useEffect(() => {
    if (chatMode !== 'group') {
      setGroupTask(null);
      return;
    }
    void loadLatestTask(selectedId);
  }, [chatMode, loadLatestTask, selectedId]);

  useEffect(() => {
    if (chatMode !== 'group' || !processing || !selectedId) return;
    const timer = window.setInterval(() => {
      void loadLatestTask(selectedId);
    }, 2500);
    return () => window.clearInterval(timer);
  }, [chatMode, loadLatestTask, processing, selectedId]);

  useEffect(() => {
    setMessages([]);
    setPendingUserMessage(null);
    setErrorText('');
  }, [chatMode]);

  useLayoutEffect(() => {
    const container = messageListRef.current;
    if (!container) return;
    container.scrollTop = container.scrollHeight;
  }, [messages, pendingUserMessage, processing]);

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

  const createSession = useCallback(async (agentOverride?: string) => {
    if (creating) return '';
    setCreating(true);
    setErrorText('');
    try {
      const agentId = (agentOverride || selectedAgentId || agents[0]?.id || 'main').trim();
      const res = await api.createPanelChatSession({ chatType: 'direct', agentId });
      if (!res?.ok || !res.session?.id) {
        setErrorText(text.failedCreate);
        return '';
      }
      await loadSessions(res.session.id);
      setSelectedId(res.session.id);
      setMessages([]);
      return res.session.id as string;
    } finally {
      setCreating(false);
    }
  }, [agents, creating, loadSessions, selectedAgentId, text.failedCreate]);

  const createGroupSession = useCallback(async () => {
    const participantAgentIds = Array.from(new Set(groupDraft.agentIds.filter(Boolean)));
    if (participantAgentIds.length === 0) {
      setErrorText(text.failedCreate);
      return '';
    }
    const controllerAgentId = participantAgentIds.includes('main') ? 'main' : participantAgentIds[0];
    const preferredAgentId = participantAgentIds.includes(selectedGroupAgentId) ? selectedGroupAgentId : controllerAgentId;
    setCreating(true);
    setErrorText('');
    try {
      const res = await api.createPanelChatSession({
        title: groupDraft.title,
        chatType: 'group',
        agentId: controllerAgentId,
        participantAgentIds,
        controllerAgentId,
        preferredAgentId,
      } as any);
      if (!res?.ok || !res.session?.id) {
        setErrorText(text.failedCreate);
        return '';
      }
      await loadSessions(res.session.id);
      setSelectedId(res.session.id);
      setMessages([]);
      setSelectedGroupAgentId(preferredAgentId);
      setGroupDraft({ open: false, title: '', agentIds: participantAgentIds });
      return res.session.id as string;
    } finally {
      setCreating(false);
    }
  }, [groupDraft, loadSessions, selectedGroupAgentId, text.failedCreate]);

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
    const rawMessage = input.trim();
    const message = rawMessage;
    if (!message || loading) return;
    let sessionId = selectedId;
    const requestId = activeRequestIdRef.current + 1;
    activeRequestIdRef.current = requestId;
    setErrorText('');
    setLoading(true);
    setInput('');
    try {
      if (!sessionId) {
        sessionId = chatMode === 'group' ? await createGroupSession() : await createSession();
      }
      if (!sessionId) return;
      const effectivePreferredAgentId = chatMode === 'group' ? selectedGroupAgentId : '';
      setPendingUserMessage({
        id: `pending-user-${Date.now()}`,
        role: 'user',
        content: rawMessage,
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
        body: JSON.stringify(chatMode === 'group' ? { message, preferredAgentId: effectivePreferredAgentId } : { message }),
        signal: controller.signal,
      });
      const res = await response.json();
      abortControllerRef.current = null;
      if (activeRequestIdRef.current !== requestId) return;
      if (res?.ok) {
        abortMarkerHandledRef.current[sessionId] = false;
        if (chatMode === 'group' && effectivePreferredAgentId) {
          setSelectedGroupAgentId(effectivePreferredAgentId);
        }
        if (selectedIdRef.current === sessionId) {
          const nextMessages = Array.isArray(res.messages) ? [...res.messages] : [];
          if (chatMode === 'group' && nextMessages.length > 0) {
            for (let i = nextMessages.length - 1; i >= 0; i -= 1) {
              if (nextMessages[i]?.role === 'user') {
                nextMessages[i] = { ...nextMessages[i], content: rawMessage };
                break;
              }
            }
          }
          setMessages(nextMessages);
          setPendingUserMessage(null);
        }
        await loadSessions(sessionId);
      } else if (res?.canceled) {
        appendAbortMarker(sessionId);
        await loadSessions(sessionId);
      } else {
        if (selectedIdRef.current === sessionId) {
          setPendingUserMessage(null);
        }
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
        if (!processingSessionId || processingSessionId === sessionId) {
          abortMarkerHandledRef.current[sessionId] = false;
        }
        setProcessingSessionId(current => current === sessionId ? '' : current);
      }
      setLoading(false);
    }
  }, [agents, appendAbortMarker, chatMode, createGroupSession, createSession, input, loadSessions, loading, selectedGroupAgentId, selectedId, text.failedSend]);

  const handleDelete = useCallback(async () => {
    if (!selectedSession || interactionLocked || !window.confirm(text.deleteConfirm)) return;
    const deletingId = selectedSession.id;
    const pool = chatMode === 'group' ? groupSessions : directSessions;
    const fallback = pool.find(item => item.id !== deletingId)?.id || '';
    const res = await api.deletePanelChatSession(deletingId);
    if (!res?.ok) {
      setErrorText(text.failedDelete);
      return;
    }
    setErrorText('');
    setSelectedId(fallback);
    if (!fallback) setMessages([]);
    await loadSessions(fallback);
  }, [chatMode, directSessions, groupSessions, interactionLocked, loadSessions, selectedSession, text.deleteConfirm, text.failedDelete]);

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
  }, [chatMode, draftTitle, loadSessions, selectedSession, text.failedRename]);

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
      try {
        const textarea = document.createElement('textarea');
        textarea.value = content;
        textarea.setAttribute('readonly', 'true');
        textarea.style.position = 'fixed';
        textarea.style.top = '-1000px';
        textarea.style.opacity = '0';
        document.body.appendChild(textarea);
        textarea.focus();
        textarea.select();
        textarea.setSelectionRange(0, textarea.value.length);
        const copied = document.execCommand('copy');
        document.body.removeChild(textarea);
        if (!copied) throw new Error('execCommand failed');
        setCopiedCode(content);
        window.setTimeout(() => {
          setCopiedCode(current => current === content ? '' : current);
        }, 1800);
      } catch {
        setErrorText(locale === 'en' ? 'Copy failed.' : '复制失败。');
      }
    }
  }, [locale]);

  const handleCreateSessionWithSelectedAgent = useCallback(async () => {
    const nextId = await createSession(selectedAgentId);
    if (nextId) {
      setChatMode('direct');
      setSelectedId(nextId);
      setMessages([]);
      setPendingUserMessage(null);
    }
  }, [createSession, selectedAgentId]);

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
            <div className="flex items-center justify-between gap-3">
              <div className="flex gap-2 text-xs">
                <span className="inline-flex items-center gap-1.5 rounded-xl border border-blue-200 bg-blue-50 px-3 py-2 text-xs text-blue-700 dark:border-blue-500/30 dark:bg-blue-500/10 dark:text-blue-200">{text.direct}</span>
              </div>
              <button onClick={() => void createSession()} disabled={interactionLocked} className="inline-flex items-center gap-1.5 rounded-xl bg-slate-900 px-3 py-2 text-xs font-medium text-white transition hover:bg-slate-800 disabled:cursor-not-allowed disabled:opacity-60 dark:bg-white dark:text-slate-900 dark:hover:bg-slate-100">
                {creating ? <Loader2 size={14} className="animate-spin" /> : <MessageSquarePlus size={14} />}
                {text.newChat}
              </button>
            </div>
            <div className="mt-3 flex items-center gap-2 text-xs text-slate-500 dark:text-slate-400">
              <span className="font-semibold text-slate-900 dark:text-slate-100 tabular-nums">{displayedSessions.length}</span>
              <span>会话</span>
            </div>
            {chatMode === 'direct' && (
              <div className="mt-3 rounded-2xl border border-slate-200/80 bg-white/80 px-3 py-3 dark:border-slate-700 dark:bg-slate-950/50">
                <label className="mb-2 block text-[11px] font-medium uppercase tracking-[0.16em] text-slate-400 dark:text-slate-500">{text.agentForNewChat}</label>
                <select
                  value={selectedAgentId}
                  onChange={event => setSelectedAgentId(event.target.value)}
                  className="w-full rounded-xl border border-slate-200 bg-white px-3 py-2 text-sm text-slate-700 outline-none transition focus:border-blue-300 dark:border-slate-700 dark:bg-slate-900 dark:text-slate-100"
                >
                  {agents.map(agent => (
                    <option key={agent.id} value={agent.id}>{agent.name ? `${agent.name} (${agent.id})` : agent.id}{agent.isDefault ? ` · ${text.defaultAgentSuffix}` : ''}</option>
                  ))}
                </select>
                <button
                  type="button"
                  onClick={() => void handleCreateSessionWithSelectedAgent()}
                  disabled={interactionLocked || !selectedAgentId}
                  className="mt-2 inline-flex w-full items-center justify-center gap-2 rounded-xl border border-slate-200 px-3 py-2 text-sm text-slate-600 transition hover:bg-slate-50 disabled:cursor-not-allowed disabled:opacity-50 dark:border-slate-700 dark:text-slate-300 dark:hover:bg-slate-900"
                >
                  {text.openWithAgent}
                  {selectedDraftAgentMeta && <span className={`rounded-full px-2 py-0.5 text-[11px] font-semibold ${agentBadgeTone(selectedDraftAgentMeta.id)}`}>{agentDisplayName(selectedDraftAgentMeta)}</span>}
                </button>
              </div>
            )}
          </div>
          <div className="min-h-0 flex-1 overflow-y-auto bg-slate-50/70 p-3 dark:bg-slate-950/30">
            {booting ? (
              <div className="flex h-full items-center justify-center text-sm text-slate-400"><Loader2 size={16} className="mr-2 animate-spin" />{text.loading}</div>
            ) : displayedSessions.length === 0 ? (
              <div className="flex h-full items-center justify-center px-4 text-center text-sm text-slate-400">{text.noSessions}</div>
            ) : (
              <div className="space-y-2">
                {displayedSessions.map(session => (
                  <button
                    key={session.id}
                    onClick={() => {
                      if (sessionSwitchLocked) return;
                      if (session.id !== selectedId) {
                        detailRequestIdRef.current += 1;
                        setMessages([]);
                        setPendingUserMessage(null);
                      }
                      setErrorText('');
                      setSelectedId(session.id);
                    }}
                    disabled={sessionSwitchLocked}
                    className={`w-full rounded-2xl border px-3 py-3 text-left transition ${selectedId === session.id ? 'border-blue-200 bg-blue-50 shadow-sm ring-1 ring-blue-100 dark:border-blue-500/40 dark:bg-blue-500/12 dark:ring-blue-500/20' : 'border-transparent bg-white/75 hover:border-slate-200 hover:bg-white dark:bg-slate-900/30 dark:hover:border-slate-800 dark:hover:bg-slate-900/55'}`}
                  >
                    <div className="flex items-center justify-between gap-2">
                      <span className={`truncate text-sm font-semibold ${selectedId === session.id ? 'text-blue-900 dark:text-blue-100' : 'text-slate-800 dark:text-slate-100'}`}>{session.title || text.newChat}</span>
                      <span className="rounded-full border border-slate-200 bg-white px-2 py-0.5 text-[10px] text-slate-500 dark:border-slate-600 dark:bg-transparent dark:text-slate-300">{session.chatType === 'group' ? <Users size={12} className="inline-block" /> : <User size={12} className="inline-block" />}</span>
                    </div>
                    <p className={`mt-1 line-clamp-2 text-xs ${selectedId === session.id ? 'text-blue-700 dark:text-blue-200' : 'text-slate-500 dark:text-slate-400'}`}>{session.processing ? text.processing : (session.lastMessage || `${text.agentLabel}: ${agentDisplayName(agents.find(item => item.id === session.agentId)) || session.agentId}`)}</p>
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
              <div className="mt-2 flex flex-wrap items-center gap-2 text-xs text-slate-500 dark:text-slate-400">
                {selectedSession ? (
                  selectedSession.chatType === 'group' ? (
                    <>
                      <span className="rounded-full border border-slate-200 bg-white px-2.5 py-1 dark:border-slate-700 dark:bg-slate-900">{statusLabel(selectedSession.status, locale)}</span>
                      <span className="text-slate-400 dark:text-slate-500">主控</span>
                      <span className={`rounded-full px-2.5 py-1 font-semibold ${agentBadgeTone(selectedSession.controllerAgentId || selectedSession.agentId)}`}>{agentDisplayName(agents.find(item => item.id === (selectedSession.controllerAgentId || selectedSession.agentId))) || selectedSession.controllerAgentId || selectedSession.agentId}</span>
                      {selectedSession.participantAgentIds && selectedSession.participantAgentIds.filter(agentId => agentId !== (selectedSession.controllerAgentId || selectedSession.agentId)).length > 0 && (
                        <>
                          <span className="text-slate-400 dark:text-slate-500">参与</span>
                          {selectedSession.participantAgentIds.filter(agentId => agentId !== (selectedSession.controllerAgentId || selectedSession.agentId)).map(agentId => {
                            const agentMeta = agents.find(item => item.id === agentId);
                            return <span key={agentId} className={`rounded-full px-2.5 py-1 font-semibold ${agentBadgeTone(agentId)}`}>{agentDisplayName(agentMeta) || agentId}</span>;
                          })}
                        </>
                      )}
                    </>
                  ) : (
                    <>
                      <span>{text.agentLabel}:</span>
                      <span className={`rounded-full px-2.5 py-1 font-semibold ${agentBadgeTone(selectedSession.agentId)}`}>{selectedAgentMeta?.name ? `${selectedAgentMeta.name} (${selectedSession.agentId})` : selectedSession.agentId}</span>
                      <span className="rounded-full border border-slate-200 bg-white px-2.5 py-1 dark:border-slate-700 dark:bg-slate-900">{statusLabel(selectedSession.status, locale)}</span>
                    </>
                  )
                ) : (
                  <span>{text.emptyDesc}</span>
                )}
              </div>
              {processing && <p className="mt-1 text-xs font-medium text-blue-600 dark:text-blue-300">{text.processing}</p>}
            </div>
              <div className="flex items-center gap-2">
                {chatMode === 'group' && (
                  <button type="button" onClick={() => setShowGroupTaskCard(value => !value)} className="inline-flex items-center gap-1.5 rounded-xl border border-slate-200 px-3 py-2 text-xs text-slate-600 transition hover:bg-slate-50 dark:border-slate-700 dark:text-slate-300 dark:hover:bg-slate-900">
                    {showGroupTaskCard ? <ChevronUp size={14} /> : <ChevronDown size={14} />}
                    {showGroupTaskCard ? text.hideTaskCard : text.showTaskCard}
                  </button>
                )}
                <button onClick={() => setRenaming(value => !value)} disabled={!selectedSession || interactionLocked} className="inline-flex items-center gap-1.5 rounded-xl border border-slate-200 px-3 py-2 text-xs text-slate-600 transition hover:bg-slate-50 disabled:cursor-not-allowed disabled:opacity-40 dark:border-slate-700 dark:text-slate-300 dark:hover:bg-slate-900">{text.rename}</button>
                <button onClick={handleDelete} disabled={!selectedSession || interactionLocked} className="inline-flex items-center gap-1.5 rounded-xl border border-slate-200 px-3 py-2 text-xs text-slate-600 transition hover:bg-slate-50 disabled:cursor-not-allowed disabled:opacity-40 dark:border-slate-700 dark:text-slate-300 dark:hover:bg-slate-900">
                  <Trash2 size={14} />
                  {text.delete}
                </button>
            </div>
          </div>

          <div ref={messageListRef} className="min-h-0 flex-1 overflow-y-auto bg-[radial-gradient(circle_at_top,rgba(59,130,246,0.08),transparent_24%),linear-gradient(180deg,rgba(255,255,255,0.16),transparent_36%)] px-5 py-5 dark:bg-[radial-gradient(circle_at_top,rgba(59,130,246,0.14),transparent_28%),linear-gradient(180deg,rgba(255,255,255,0.03),transparent_36%)]">
            {chatMode === 'group' && selectedSession && (
              <div className="sticky top-0 z-10 mb-5 space-y-3 bg-transparent pb-2">
                {showGroupTaskCard && (
              <div className="rounded-3xl border border-slate-200/80 bg-white/95 p-4 shadow-sm backdrop-blur-sm dark:border-slate-700/80 dark:bg-slate-950/90">
                <div className="flex flex-wrap items-start justify-between gap-4">
                  <div className="min-w-0 flex-1">
                    <div className="text-[11px] font-medium uppercase tracking-[0.16em] text-slate-400 dark:text-slate-500">{text.taskCard}</div>
                    <p className="mt-2 line-clamp-3 text-sm leading-6 text-slate-700 dark:text-slate-200">{latestGroupUserTask || '当前群聊会话正在等待新的协作任务。'}</p>
                  </div>
                  <div className="flex flex-wrap items-center gap-2 text-xs">
                    <span className="text-slate-400">主控</span>
                    <span className={`rounded-full px-2.5 py-1 font-semibold ${agentBadgeTone(selectedSession.controllerAgentId || selectedSession.agentId)}`}>{agentDisplayName(agents.find(item => item.id === (selectedSession.controllerAgentId || selectedSession.agentId))) || selectedSession.controllerAgentId || selectedSession.agentId}</span>
                  </div>
                </div>
                <div className="mt-4 grid gap-3 sm:grid-cols-5">
                  {groupProgress.steps.map((step, index) => {
                    const active = index === groupProgress.activeIndex;
                    const done = index < groupProgress.activeIndex || (!processing && index === groupProgress.activeIndex && groupProgress.activeIndex > 0);
                    return (
                      <div key={step.id} className={`rounded-2xl border px-3 py-3 text-center transition ${active ? 'border-blue-200 bg-blue-50 text-blue-700 dark:border-blue-500/30 dark:bg-blue-500/10 dark:text-blue-200' : done ? 'border-emerald-200 bg-emerald-50 text-emerald-700 dark:border-emerald-500/30 dark:bg-emerald-500/10 dark:text-emerald-200' : 'border-slate-200 bg-slate-50 text-slate-400 dark:border-slate-700 dark:bg-slate-900/70 dark:text-slate-500'}`}>
                        <div className="mx-auto mb-2 flex h-7 w-7 items-center justify-center rounded-full border border-current text-[11px] font-semibold">{index + 1}</div>
                        <div className="text-xs font-medium">{step.label}</div>
                      </div>
                    );
                  })}
                </div>
                <div className="mt-4 flex flex-wrap items-center gap-2 text-xs">
                  {[['all', text.viewAll], ['internal', text.viewInternal], ['final', text.viewFinal]].map(([value, label]) => (
                    <button key={value} type="button" onClick={() => setGroupMessageView(value as GroupMessageView)} className={`rounded-xl border px-3 py-1.5 transition ${groupMessageView === value ? 'border-blue-200 bg-blue-50 text-blue-700 dark:border-blue-500/30 dark:bg-blue-500/10 dark:text-blue-200' : 'border-slate-200 bg-white text-slate-500 dark:border-slate-700 dark:bg-slate-950 dark:text-slate-400'}`}>{label}</button>
                  ))}
                </div>
                {groupTask && (
                  <div className="mt-4 max-h-[56vh] overflow-y-auto pr-1">
                  <div className="space-y-3 rounded-2xl border border-slate-200 bg-slate-50/80 p-3 dark:border-slate-700 dark:bg-slate-900/60">
                      <div className="text-[11px] font-medium uppercase tracking-[0.16em] text-slate-400 dark:text-slate-500">子任务状态</div>
                      <div className="grid gap-2 sm:grid-cols-2 xl:grid-cols-3">
                        {Object.values(groupTask.subtasks || {}).map(subtask => {
                          const agentMeta = agents.find(item => item.id === subtask.agentId);
                          return (
                            <div key={subtask.agentId} className="rounded-xl border border-slate-200 bg-white px-3 py-3 text-xs dark:border-slate-700 dark:bg-slate-950/80">
                              <div className="flex items-center justify-between gap-2">
                                <span className={`rounded-full px-2 py-1 font-semibold ${agentBadgeTone(subtask.agentId)}`}>{agentDisplayName(agentMeta) || subtask.agentId}</span>
                                <span className="rounded-full border border-slate-200 px-2 py-1 text-[10px] text-slate-500 dark:border-slate-700 dark:text-slate-400">{subtask.status}</span>
                              </div>
                              {subtask.summary && <p className="mt-2 leading-5 text-slate-500 dark:text-slate-400">{subtask.summary}</p>}
                            </div>
                          );
                        })}
                      </div>
                  </div>
                  </div>
                )}
              </div>
                )}
              </div>
            )}
            {filteredTimelineMessages.length === 0 ? (
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
              <div className="flex min-h-full flex-col justify-end">
                <div className="space-y-4">
                {filteredTimelineMessages.map((message, index) => {
                  const isUser = message.role === 'user';
                  const isSystem = message.role === 'system';
                  const isInternal = !!message.internal;
                  const showStageDivider = chatMode === 'group' && !!message.stage && (index === 0 || filteredTimelineMessages[index - 1]?.stage !== message.stage);
                  return (
                    <Fragment key={message.id}>
                    {showStageDivider && (
                      <div key={`${message.id}-stage`} className="my-2 flex items-center gap-3 text-[11px] font-medium uppercase tracking-[0.16em] text-slate-400 dark:text-slate-500">
                        <div className="h-px flex-1 bg-slate-200 dark:bg-slate-700" />
                        <span>{stageLabel(message.stage, locale)}</span>
                        <div className="h-px flex-1 bg-slate-200 dark:bg-slate-700" />
                      </div>
                    )}
                    <div className={`flex gap-3 ${isSystem ? 'justify-center' : isUser ? 'justify-end' : 'justify-start'}`}>
                      {isSystem ? (
                        <div className="my-2 flex w-full items-center gap-3 text-xs text-slate-400 dark:text-slate-500">
                          <div className="h-px flex-1 bg-slate-200 dark:bg-slate-700" />
                          <span className="shrink-0 rounded-full border border-slate-200 bg-white px-3 py-1 font-medium dark:border-slate-700 dark:bg-slate-900">{message.content}</span>
                          <div className="h-px flex-1 bg-slate-200 dark:bg-slate-700" />
                        </div>
                      ) : (
                        <>
                      {!isUser && <div className="mt-1 flex h-8 w-8 shrink-0 items-center justify-center rounded-full border border-slate-200 bg-white text-slate-500 dark:border-slate-700 dark:bg-slate-900 dark:text-slate-300"><Bot size={15} /></div>}
                      <div className={`max-w-[85%] rounded-2xl px-4 py-3 text-sm leading-6 shadow-sm transition ${isUser ? 'rounded-tr-sm bg-[linear-gradient(135deg,#1d4ed8,#0284c7)] text-right text-white shadow-blue-200/50 dark:shadow-none' : isInternal ? 'rounded-tl-sm border border-amber-200/80 bg-amber-50 text-slate-700 dark:border-amber-500/20 dark:bg-slate-900 dark:text-slate-200' : 'rounded-tl-sm border border-slate-200/70 bg-white text-slate-700 dark:border-slate-700 dark:bg-slate-950 dark:text-slate-200'} ${highlightedId === message.id ? 'ring-2 ring-blue-300 dark:ring-blue-500/50' : ''}`}>
                        {!isUser && chatMode === 'group' && message.agentId && (
                          <div className="mb-2 flex items-center gap-2 text-[11px]">
                            <span className={`rounded-full px-2.5 py-1 font-semibold ${agentBadgeTone(message.agentId)}`}>{message.agentId}</span>
                            {isInternal && <span className="rounded-full border border-amber-200 bg-white px-2 py-0.5 text-[10px] text-amber-700 dark:border-amber-500/30 dark:bg-transparent dark:text-amber-200">内部协作</span>}
                            {message.agentId === (agents[0]?.id || 'main') && <span className="text-slate-400 dark:text-slate-500">{locale === 'en' ? 'Lead Agent' : '主 Agent'}</span>}
                          </div>
                        )}
                        {isUser ? message.content : (
                          <>
                            {message.content && (
                              <ReactMarkdown
                                remarkPlugins={[remarkGfm]}
                                components={{
                                  p: ({ children }) => <p className="mb-2 last:mb-0">{children}</p>,
                                  hr: () => <hr className="my-4 border-slate-200 dark:border-slate-700" />,
                                  table: ({ children }) => <div className="my-3 overflow-x-auto rounded-xl border border-slate-200 dark:border-slate-700"><table className="min-w-full border-collapse text-left text-[13px]">{children}</table></div>,
                                  thead: ({ children }) => <thead className="bg-slate-100 dark:bg-slate-900">{children}</thead>,
                                  th: ({ children }) => <th className="border-b border-slate-200 px-3 py-2 font-semibold text-slate-700 dark:border-slate-700 dark:text-slate-100">{children}</th>,
                                  td: ({ children }) => <td className="border-b border-slate-200 px-3 py-2 text-slate-600 dark:border-slate-800 dark:text-slate-300">{children}</td>,
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
                                            onClick={(event) => {
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
                                {formatDiagramBlocks(message.content)}
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
                      <div className="flex items-center gap-1.5 text-blue-500 dark:text-blue-300">
                        <span className="h-2 w-2 rounded-full bg-current animate-bounce [animation-delay:-0.3s]" />
                        <span className="h-2 w-2 rounded-full bg-current animate-bounce [animation-delay:-0.15s]" />
                        <span className="h-2 w-2 rounded-full bg-current animate-bounce" />
                      </div>
                    </div>
                  </div>
                )}
                </div>
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
                    handleSend();
                  }
                }}
                rows={3}
                placeholder={chatMode === 'group' ? text.groupInput : text.input}
                className="w-full resize-none bg-transparent px-3 py-2 text-sm outline-none placeholder:text-slate-400 dark:text-slate-100"
              />
              <div className="flex items-center justify-between px-2 pb-1 pt-2">
                <div className="flex items-center gap-3 text-xs text-slate-400">
                  <span>{text.enterHint}</span>
                  {chatMode === 'group' && selectedSession?.participantAgentIds && selectedSession.participantAgentIds.length > 0 && (
                    <label className="flex items-center gap-2 text-slate-500 dark:text-slate-400">
                      <span>{text.groupReplyAgent}</span>
                      <select value={selectedGroupAgentId} onChange={event => setSelectedGroupAgentId(event.target.value)} className="rounded-lg border border-slate-200 bg-white px-2 py-1 text-xs text-slate-700 outline-none dark:border-slate-700 dark:bg-slate-900 dark:text-slate-100">
                        {selectedSession.participantAgentIds.map(agentId => {
                          const agentMeta = agents.find(item => item.id === agentId);
                          return <option key={agentId} value={agentId}>{agentDisplayName(agentMeta) || agentId}</option>;
                        })}
                      </select>
                    </label>
                  )}
                </div>
                <button onClick={loading ? handleAbort : handleSend} disabled={creating || (!loading && !input.trim())} className={`inline-flex items-center gap-2 rounded-2xl px-4 py-2 text-sm font-medium text-white transition disabled:cursor-not-allowed disabled:opacity-50 ${loading ? 'bg-rose-600 hover:bg-rose-500 dark:bg-rose-500 dark:text-white dark:hover:bg-rose-400' : 'bg-slate-900 hover:bg-slate-800 dark:bg-white dark:text-slate-900 dark:hover:bg-slate-100'}`}>
                  {loading ? <Square size={15} fill="currentColor" /> : <Send size={16} />}
                  {loading ? text.stop : text.send}
                </button>
              </div>
            </div>
          </div>
        </div>
      </section>

      {groupDraft.open && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-slate-950/45 px-4">
          <div className="w-full max-w-xl rounded-3xl border border-slate-200 bg-white p-5 shadow-2xl dark:border-slate-700 dark:bg-slate-950">
            <div className="flex items-start justify-between gap-4">
              <div>
                <h3 className="text-lg font-semibold text-slate-900 dark:text-slate-50">{text.createGroupTitle}</h3>
                <p className="mt-1 text-sm text-slate-500 dark:text-slate-400">{text.groupParticipantHint}</p>
              </div>
              <button type="button" onClick={() => setGroupDraft(current => ({ ...current, open: false }))} className="rounded-xl border border-slate-200 px-3 py-2 text-xs text-slate-500 dark:border-slate-700 dark:text-slate-300">关闭</button>
            </div>
            <div className="mt-4 space-y-4">
              <div>
                <label className="mb-2 block text-xs text-slate-500 dark:text-slate-400">{text.renamePlaceholder}</label>
                <input value={groupDraft.title} onChange={event => setGroupDraft(current => ({ ...current, title: event.target.value }))} className="w-full rounded-xl border border-slate-200 bg-white px-3 py-2.5 text-sm text-slate-700 outline-none dark:border-slate-700 dark:bg-slate-900 dark:text-slate-100" placeholder={text.createGroupTitle} />
              </div>
              <div className="grid gap-2 sm:grid-cols-2">
                {agents.map(agent => {
                  const checked = groupDraft.agentIds.includes(agent.id);
                  return (
                    <label key={agent.id} className={`flex cursor-pointer items-center justify-between gap-3 rounded-2xl border px-3 py-3 ${checked ? 'border-blue-200 bg-blue-50 dark:border-blue-500/30 dark:bg-blue-500/10' : 'border-slate-200 dark:border-slate-700'}`}>
                      <div>
                        <div className="text-sm font-medium text-slate-900 dark:text-slate-100">{agentDisplayName(agent)}</div>
                        {agent.isDefault && <div className="mt-1 text-[11px] text-slate-400 dark:text-slate-500">{text.defaultAgentSuffix}</div>}
                      </div>
                      <input
                        type="checkbox"
                        checked={checked}
                        onChange={() => setGroupDraft(current => {
                          const exists = current.agentIds.includes(agent.id);
                          const agentIds = exists ? current.agentIds.filter(item => item !== agent.id) : [...current.agentIds, agent.id];
                          return { ...current, agentIds: agentIds.length > 0 ? agentIds : ['main'] };
                        })}
                        className="h-4 w-4"
                      />
                    </label>
                  );
                })}
              </div>
              <div className="flex justify-end gap-2">
                <button type="button" onClick={() => setGroupDraft(current => ({ ...current, open: false }))} className="rounded-xl border border-slate-200 px-4 py-2 text-sm text-slate-600 dark:border-slate-700 dark:text-slate-300">取消</button>
                <button type="button" onClick={() => void createGroupSession()} disabled={creating || groupDraft.agentIds.length === 0} className="rounded-xl bg-slate-900 px-4 py-2 text-sm font-medium text-white disabled:opacity-50 dark:bg-white dark:text-slate-900">{text.createGroupConfirm}</button>
              </div>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
