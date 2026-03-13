import { Routes, Route, Navigate } from 'react-router-dom';
import { useAuth } from './hooks/useAuth';
import { useWebSocket } from './hooks/useWebSocket';
import Layout from './components/Layout';
import OpenClawRequired from './components/OpenClawRequired';
import UpdatePopup from './components/UpdatePopup';
import Login from './pages/Login';
import Dashboard from './pages/Dashboard';
import ActivityLog from './pages/ActivityLog';
import Channels from './pages/Channels';
import HelpDocs from './pages/HelpDocs';
import Skills from './pages/Skills';
import CronJobs from './pages/CronJobs';
import SystemConfig from './pages/SystemConfig';
import Sessions from './pages/Sessions';
import Workspace from './pages/Workspace';
import Plugins from './pages/Plugins';
import Agents from './pages/Agents';
import Workflows from './pages/Workflows';

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
        <Route path="/" element={<Dashboard ws={ws} />} />
        <Route path="/logs" element={<ActivityLog ws={ws} />} />
        <Route path="/channels" element={<OpenClawRequired openclawStatus={ws.openclawStatus} processStatus={ws.processStatus}><Channels /></OpenClawRequired>} />
        <Route path="/skills" element={<OpenClawRequired openclawStatus={ws.openclawStatus} processStatus={ws.processStatus}><Skills /></OpenClawRequired>} />
        <Route path="/docs" element={<HelpDocs />} />
        <Route path="/plugins" element={<OpenClawRequired openclawStatus={ws.openclawStatus} processStatus={ws.processStatus}><Plugins /></OpenClawRequired>} />
        {enableAgents && (
          <Route path="/agents" element={<OpenClawRequired openclawStatus={ws.openclawStatus} processStatus={ws.processStatus}><Agents /></OpenClawRequired>} />
        )}
        <Route path="/workflows" element={<OpenClawRequired openclawStatus={ws.openclawStatus} processStatus={ws.processStatus}><Workflows /></OpenClawRequired>} />
        <Route path="/cron" element={<OpenClawRequired openclawStatus={ws.openclawStatus} processStatus={ws.processStatus}><CronJobs /></OpenClawRequired>} />
        <Route path="/sessions" element={<OpenClawRequired openclawStatus={ws.openclawStatus} processStatus={ws.processStatus}><Sessions /></OpenClawRequired>} />
        <Route path="/config" element={<SystemConfig />} />
        <Route path="/workspace" element={<Workspace />} />
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
