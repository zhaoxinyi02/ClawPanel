import { Routes, Route, Navigate } from 'react-router-dom';
import { useAuth } from './hooks/useAuth';
import { useWebSocket } from './hooks/useWebSocket';
import Layout from './components/Layout';
import Login from './pages/Login';
import Dashboard from './pages/Dashboard';
import ActivityLog from './pages/ActivityLog';
import Channels from './pages/Channels';
import Skills from './pages/Skills';
import CronJobs from './pages/CronJobs';
import SystemConfig from './pages/SystemConfig';
import Workspace from './pages/Workspace';
import Requests from './pages/Requests';

export default function App() {
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
    <Routes>
      <Route element={<Layout onLogout={auth.logout} napcatStatus={ws.napcatStatus} wechatStatus={ws.wechatStatus} />}>
        <Route path="/" element={<Dashboard ws={ws} />} />
        <Route path="/logs" element={<ActivityLog ws={ws} />} />
        <Route path="/channels" element={<Channels />} />
        <Route path="/skills" element={<Skills />} />
        <Route path="/cron" element={<CronJobs />} />
        <Route path="/config" element={<SystemConfig />} />
        <Route path="/workspace" element={<Workspace />} />
        <Route path="/requests" element={<Requests />} />
      </Route>
      <Route path="/login" element={<Navigate to="/" />} />
      {/* Legacy redirects */}
      <Route path="/qq" element={<Navigate to="/channels" />} />
      <Route path="/qqbot" element={<Navigate to="/channels" />} />
      <Route path="/wechat" element={<Navigate to="/channels" />} />
      <Route path="/openclaw" element={<Navigate to="/config" />} />
      <Route path="/settings" element={<Navigate to="/config" />} />
      <Route path="*" element={<Navigate to="/" />} />
    </Routes>
  );
}
