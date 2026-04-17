import { Suspense, lazy, useEffect } from 'react';
import { Routes, Route, Navigate } from 'react-router-dom';
import { useAuth } from './hooks/useAuth';
import { useWebSocket } from './hooks/useWebSocket';
import Layout from './components/Layout';
import OpenClawRequired from './components/OpenClawRequired';
import UpdatePopup from './components/UpdatePopup';
import Login from './pages/Login';
import Dashboard from './pages/Dashboard';

const loadActivityLog = () => import('./pages/ActivityLog');
const loadPanelChat = () => import('./pages/PanelChat');
const loadChannels = () => import('./pages/Channels');
const loadCronJobs = () => import('./pages/CronJobs');
const loadSkills = () => import('./pages/Skills');
const loadSystemConfig = () => import('./pages/SystemConfig');
const loadPlugins = () => import('./pages/Plugins');
const loadAgents = () => import('./pages/Agents');
const loadWorkflows = () => import('./pages/Workflows');
const loadSessions = () => import('./pages/Sessions');
const loadTasks = () => import('./pages/Tasks');
const loadWorkspace = () => import('./pages/Workspace');
const loadMonitor = () => import('./pages/Monitor');
const loadHermesOverview = () => import('./pages/HermesOverview');
const loadHermesConfig = () => import('./pages/HermesConfig');
const loadHermesHealth = () => import('./pages/HermesHealth');
const loadHermesPlatforms = () => import('./pages/HermesPlatforms');
const loadHermesSessions = () => import('./pages/HermesSessions');
const loadHermesPersonality = () => import('./pages/HermesPersonality');
const loadHermesLogs = () => import('./pages/HermesLogs');
const loadHermesActions = () => import('./pages/HermesActions');
const loadHermesTasks = () => import('./pages/HermesTasks');
const loadHermesProfiles = () => import('./pages/HermesProfiles');

const ActivityLog = lazy(loadActivityLog);
const PanelChat = lazy(loadPanelChat);
const Channels = lazy(loadChannels);
const CronJobs = lazy(loadCronJobs);
const Skills = lazy(loadSkills);
const SystemConfig = lazy(loadSystemConfig);
const Plugins = lazy(loadPlugins);
const Agents = lazy(loadAgents);
const Workflows = lazy(loadWorkflows);
const Sessions = lazy(loadSessions);
const Tasks = lazy(loadTasks);
const Workspace = lazy(loadWorkspace);
const Monitor = lazy(loadMonitor);
const HermesOverview = lazy(loadHermesOverview);
const HermesConfig = lazy(loadHermesConfig);
const HermesHealth = lazy(loadHermesHealth);
const HermesPlatforms = lazy(loadHermesPlatforms);
const HermesSessions = lazy(loadHermesSessions);
const HermesPersonality = lazy(loadHermesPersonality);
const HermesLogs = lazy(loadHermesLogs);
const HermesActions = lazy(loadHermesActions);
const HermesTasks = lazy(loadHermesTasks);
const HermesProfiles = lazy(loadHermesProfiles);
const CompanyOverview = lazy(() => import('./pages/CompanyOverview'));
const CompanyTasks = lazy(() => import('./pages/CompanyTasks'));
const CompanyTaskDetail = lazy(() => import('./pages/CompanyTaskDetail'));
const CompanyTeams = lazy(() => import('./pages/CompanyTeams'));
const CompanyChannels = lazy(() => import('./pages/CompanyChannels'));

function RouteLoadingFallback() {
  return (
    <div className="flex min-h-[40vh] items-center justify-center px-6 py-10">
      <div
        className="inline-flex items-center gap-3 rounded-2xl border border-slate-200/70 bg-white/90 px-4 py-3 text-sm text-slate-600 shadow-sm dark:border-slate-700/70 dark:bg-slate-900/90 dark:text-slate-300"
        role="status"
        aria-live="polite"
      >
        <span className="h-2.5 w-2.5 animate-pulse rounded-full bg-blue-500" aria-hidden="true" />
        <span>Loading page…</span>
      </div>
    </div>
  );
}

const ACTIVE_AGENT_KEY = 'clawpanel-active-agent';

function getStoredAgent() {
  try {
    return localStorage.getItem(ACTIVE_AGENT_KEY) === 'hermes' ? 'hermes' : 'openclaw';
  } catch {
    return 'openclaw';
  }
}

function HomeRoute({ logEntries, refreshLog }: { logEntries: any[]; refreshLog: () => void }) {
  if (getStoredAgent() === 'hermes') {
    return <Navigate to="/hermes" replace />;
  }
  return <Dashboard logEntries={logEntries} refreshLog={refreshLog} />;
}

export default function App() {
  const enableAgents = import.meta.env.VITE_FEATURE_AGENTS !== 'false';
  const auth = useAuth();
  const ws = useWebSocket();

  useEffect(() => {
    const timer = window.setTimeout(() => {
      void loadHermesOverview();
      void loadHermesHealth();
      void loadHermesPlatforms();
      void loadHermesLogs();
      void loadHermesActions();
      void loadHermesTasks();
      void loadHermesSessions();
      void loadHermesPersonality();
      void loadHermesProfiles();
      void loadHermesConfig();
    }, 400);
    return () => window.clearTimeout(timer);
  }, []);

  if (!auth.isLoggedIn) {
    return (
      <Routes>
        <Route path="/login" element={<Login onLogin={auth.login} />} />
        <Route path="*" element={<Navigate to="/login" />} />
      </Routes>
    );
  }

  return (
    <>
    <UpdatePopup />
    <Routes>
      <Route element={<Layout onLogout={auth.logout} napcatStatus={ws.napcatStatus} wechatStatus={ws.wechatStatus} openclawStatus={ws.openclawStatus} processStatus={ws.processStatus} wsMessages={ws.wsMessages} />}>
        <Route path="/" element={<HomeRoute logEntries={ws.logEntries} refreshLog={ws.refreshLog} />} />
        <Route path="/chat" element={<OpenClawRequired openclawStatus={ws.openclawStatus} processStatus={ws.processStatus}><Suspense fallback={<RouteLoadingFallback />}><PanelChat /></Suspense></OpenClawRequired>} />
        <Route path="/logs" element={<Suspense fallback={<RouteLoadingFallback />}><ActivityLog logEntries={ws.logEntries} clearEvents={ws.clearEvents} refreshLog={ws.refreshLog} /></Suspense>} />
        <Route path="/channels" element={<OpenClawRequired openclawStatus={ws.openclawStatus} processStatus={ws.processStatus}><Suspense fallback={<RouteLoadingFallback />}><Channels /></Suspense></OpenClawRequired>} />
        <Route path="/skills" element={<OpenClawRequired openclawStatus={ws.openclawStatus} processStatus={ws.processStatus}><Suspense fallback={<RouteLoadingFallback />}><Skills /></Suspense></OpenClawRequired>} />
        <Route path="/plugins" element={<OpenClawRequired openclawStatus={ws.openclawStatus} processStatus={ws.processStatus}><Suspense fallback={<RouteLoadingFallback />}><Plugins /></Suspense></OpenClawRequired>} />
        {enableAgents && (
          <Route path="/agents" element={<OpenClawRequired openclawStatus={ws.openclawStatus} processStatus={ws.processStatus}><Suspense fallback={<RouteLoadingFallback />}><Agents /></Suspense></OpenClawRequired>} />
        )}
        {enableAgents && (
          <Route path="/monitor" element={<OpenClawRequired openclawStatus={ws.openclawStatus} processStatus={ws.processStatus}><Suspense fallback={<RouteLoadingFallback />}><Monitor /></Suspense></OpenClawRequired>} />
        )}
        <Route path="/workflows" element={<OpenClawRequired openclawStatus={ws.openclawStatus} processStatus={ws.processStatus}><Suspense fallback={<RouteLoadingFallback />}><Workflows /></Suspense></OpenClawRequired>} />
        <Route path="/company" element={<OpenClawRequired openclawStatus={ws.openclawStatus} processStatus={ws.processStatus}><Suspense fallback={<RouteLoadingFallback />}><CompanyOverview /></Suspense></OpenClawRequired>} />
        <Route path="/company/tasks" element={<OpenClawRequired openclawStatus={ws.openclawStatus} processStatus={ws.processStatus}><Suspense fallback={<RouteLoadingFallback />}><CompanyTasks /></Suspense></OpenClawRequired>} />
        <Route path="/company/tasks/:id" element={<OpenClawRequired openclawStatus={ws.openclawStatus} processStatus={ws.processStatus}><Suspense fallback={<RouteLoadingFallback />}><CompanyTaskDetail /></Suspense></OpenClawRequired>} />
        <Route path="/company/teams" element={<OpenClawRequired openclawStatus={ws.openclawStatus} processStatus={ws.processStatus}><Suspense fallback={<RouteLoadingFallback />}><CompanyTeams /></Suspense></OpenClawRequired>} />
        <Route path="/company/channels" element={<OpenClawRequired openclawStatus={ws.openclawStatus} processStatus={ws.processStatus}><Suspense fallback={<RouteLoadingFallback />}><CompanyChannels /></Suspense></OpenClawRequired>} />
        <Route path="/cron" element={<OpenClawRequired openclawStatus={ws.openclawStatus} processStatus={ws.processStatus}><Suspense fallback={<RouteLoadingFallback />}><CronJobs /></Suspense></OpenClawRequired>} />
        <Route path="/tasks" element={<OpenClawRequired openclawStatus={ws.openclawStatus} processStatus={ws.processStatus}><Suspense fallback={<RouteLoadingFallback />}><Tasks /></Suspense></OpenClawRequired>} />
        <Route path="/sessions" element={<OpenClawRequired openclawStatus={ws.openclawStatus} processStatus={ws.processStatus}><Suspense fallback={<RouteLoadingFallback />}><Sessions /></Suspense></OpenClawRequired>} />
        <Route path="/config" element={<Suspense fallback={<RouteLoadingFallback />}><SystemConfig /></Suspense>} />
        <Route path="/workspace" element={<Suspense fallback={<RouteLoadingFallback />}><Workspace /></Suspense>} />
        <Route path="/hermes" element={<Suspense fallback={<RouteLoadingFallback />}><HermesOverview /></Suspense>} />
        <Route path="/hermes/health" element={<Suspense fallback={<RouteLoadingFallback />}><HermesHealth /></Suspense>} />
        <Route path="/hermes/platforms" element={<Suspense fallback={<RouteLoadingFallback />}><HermesPlatforms /></Suspense>} />
        <Route path="/hermes/logs" element={<Suspense fallback={<RouteLoadingFallback />}><HermesLogs /></Suspense>} />
        <Route path="/hermes/actions" element={<Suspense fallback={<RouteLoadingFallback />}><HermesActions /></Suspense>} />
        <Route path="/hermes/tasks" element={<Suspense fallback={<RouteLoadingFallback />}><HermesTasks /></Suspense>} />
        <Route path="/hermes/sessions" element={<Suspense fallback={<RouteLoadingFallback />}><HermesSessions /></Suspense>} />
        <Route path="/hermes/personality" element={<Suspense fallback={<RouteLoadingFallback />}><HermesPersonality /></Suspense>} />
        <Route path="/hermes/profiles" element={<Suspense fallback={<RouteLoadingFallback />}><HermesProfiles /></Suspense>} />
        <Route path="/hermes/config" element={<Suspense fallback={<RouteLoadingFallback />}><HermesConfig /></Suspense>} />
      </Route>
      <Route path="/login" element={<Navigate to="/" />} />
      {/* Legacy redirects */}
      <Route path="/qq" element={<Navigate to="/channels" />} />
      <Route path="/qqbot" element={<Navigate to="/channels" />} />
      <Route path="/wechat" element={<Navigate to="/channels" />} />
      <Route path="/openclaw" element={<Navigate to="/config" />} />
      <Route path="/settings" element={<Navigate to="/config" />} />
      <Route path="/requests" element={<Navigate to="/channels" />} />
      <Route path="*" element={<Navigate to="/" />} />
    </Routes>
    </>
  );
}
