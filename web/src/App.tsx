import { Suspense, lazy } from 'react';
import { Routes, Route, Navigate } from 'react-router-dom';
import { useAuth } from './hooks/useAuth';
import { useWebSocket } from './hooks/useWebSocket';
import Layout from './components/Layout';
import OpenClawRequired from './components/OpenClawRequired';
import UpdatePopup from './components/UpdatePopup';
import Login from './pages/Login';
import Dashboard from './pages/Dashboard';

const ActivityLog = lazy(() => import('./pages/ActivityLog'));
const PanelChat = lazy(() => import('./pages/PanelChat'));
const Channels = lazy(() => import('./pages/Channels'));
const CronJobs = lazy(() => import('./pages/CronJobs'));
const Skills = lazy(() => import('./pages/Skills'));
const SystemConfig = lazy(() => import('./pages/SystemConfig'));
const Plugins = lazy(() => import('./pages/Plugins'));
const Agents = lazy(() => import('./pages/Agents'));
const Workflows = lazy(() => import('./pages/Workflows'));
const Sessions = lazy(() => import('./pages/Sessions'));
const Tasks = lazy(() => import('./pages/Tasks'));
const Workspace = lazy(() => import('./pages/Workspace'));
const Monitor = lazy(() => import('./pages/Monitor'));
const HermesOverview = lazy(() => import('./pages/HermesOverview'));
const HermesConfig = lazy(() => import('./pages/HermesConfig'));
const HermesHealth = lazy(() => import('./pages/HermesHealth'));
const HermesPlatforms = lazy(() => import('./pages/HermesPlatforms'));
const HermesSessions = lazy(() => import('./pages/HermesSessions'));
const HermesPersonality = lazy(() => import('./pages/HermesPersonality'));
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

export default function App() {
  const enableAgents = import.meta.env.VITE_FEATURE_AGENTS !== 'false';
  const auth = useAuth();
  const ws = useWebSocket();

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
        <Route path="/" element={<Dashboard logEntries={ws.logEntries} refreshLog={ws.refreshLog} />} />
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
        <Route path="/hermes/sessions" element={<Suspense fallback={<RouteLoadingFallback />}><HermesSessions /></Suspense>} />
        <Route path="/hermes/personality" element={<Suspense fallback={<RouteLoadingFallback />}><HermesPersonality /></Suspense>} />
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
