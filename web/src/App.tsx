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
import Skills from './pages/Skills';
import CronJobs from './pages/CronJobs';
import SystemConfig from './pages/SystemConfig';
import Sessions from './pages/Sessions';
import Workspace from './pages/Workspace';
import Plugins from './pages/Plugins';
import Agents from './pages/Agents';

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
      <Route element={<Layout onLogout={auth.logout} napcatStatus={ws.napcatStatus} wechatStatus={ws.wechatStatus} openclawStatus={ws.openclawStatus} wsMessages={ws.wsMessages} />}>
        <Route path="/" element={<Dashboard ws={ws} />} />
        <Route path="/logs" element={<ActivityLog ws={ws} />} />
        <Route path="/channels" element={<OpenClawRequired configured={!!ws.openclawStatus?.configured}><Channels /></OpenClawRequired>} />
        <Route path="/skills" element={<OpenClawRequired configured={!!ws.openclawStatus?.configured}><Skills /></OpenClawRequired>} />
        <Route path="/plugins" element={<OpenClawRequired configured={!!ws.openclawStatus?.configured}><Plugins /></OpenClawRequired>} />
        {enableAgents && (
          <Route path="/agents" element={<OpenClawRequired configured={!!ws.openclawStatus?.configured}><Agents /></OpenClawRequired>} />
        )}
        <Route path="/cron" element={<OpenClawRequired configured={!!ws.openclawStatus?.configured}><CronJobs /></OpenClawRequired>} />
        <Route path="/sessions" element={<OpenClawRequired configured={!!ws.openclawStatus?.configured}><Sessions /></OpenClawRequired>} />
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
