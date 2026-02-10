import { useEffect, useState } from 'react';
import { api } from '../lib/api';
import { CheckCircle, RefreshCw, Loader2, ExternalLink, MessageCircle } from 'lucide-react';

export default function WeChatLogin() {
  const [status, setStatus] = useState<any>(null);
  const [loginUrl, setLoginUrl] = useState('');
  const [loading, setLoading] = useState(true);
  const [sendTo, setSendTo] = useState('');
  const [sendMsg, setSendMsg] = useState('');
  const [sendResult, setSendResult] = useState('');

  const checkStatus = async () => {
    setLoading(true);
    try {
      const [s, u] = await Promise.all([api.wechatStatus(), api.wechatLoginUrl()]);
      if (s.ok) setStatus(s);
      if (u.ok) setLoginUrl(u.externalUrl || '');
    } catch {}
    setLoading(false);
  };

  useEffect(() => {
    checkStatus();
    const t = setInterval(checkStatus, 8000);
    return () => clearInterval(t);
  }, []);

  const isLoggedIn = status?.loggedIn === true;

  const handleSend = async () => {
    if (!sendTo || !sendMsg) return;
    setSendResult('');
    try {
      const r = await api.wechatSend(sendTo, sendMsg);
      setSendResult(r.ok ? '发送成功' : (r.error || '发送失败'));
      if (r.ok) { setSendMsg(''); }
    } catch (e) {
      setSendResult('发送失败: ' + String(e));
    }
  };

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h2 className="text-lg font-bold">微信登录</h2>
        <button onClick={checkStatus} className="btn-secondary text-xs py-1.5 px-3 flex items-center gap-1.5">
          <RefreshCw size={14} />刷新状态
        </button>
      </div>

      {loading && !status ? (
        <div className="card p-6 flex items-center justify-center">
          <Loader2 size={32} className="animate-spin text-gray-400" />
        </div>
      ) : isLoggedIn ? (
        <>
          <div className="card p-6 text-center space-y-3">
            <CheckCircle size={48} className="mx-auto text-emerald-500" />
            <h3 className="text-lg font-bold text-emerald-600">微信已登录</h3>
            {status?.name && (
              <p className="text-sm text-gray-600 dark:text-gray-400">昵称: {status.name}</p>
            )}
          </div>

          <div className="card p-4 space-y-3">
            <h3 className="font-semibold text-sm flex items-center gap-2">
              <MessageCircle size={16} />发送测试消息
            </h3>
            <input value={sendTo} onChange={e => setSendTo(e.target.value)}
              placeholder="接收者昵称或备注名" className="input" />
            <input value={sendMsg} onChange={e => setSendMsg(e.target.value)}
              placeholder="消息内容" className="input"
              onKeyDown={e => e.key === 'Enter' && handleSend()} />
            {sendResult && (
              <p className={`text-sm ${sendResult.includes('成功') ? 'text-emerald-600' : 'text-red-500'}`}>{sendResult}</p>
            )}
            <button onClick={handleSend} disabled={!sendTo || !sendMsg} className="btn-primary text-xs py-1.5 px-4">
              发送
            </button>
          </div>
        </>
      ) : (
        <div className="card p-6 space-y-4">
          <div className="space-y-3">
            <p className="text-sm text-gray-600 dark:text-gray-400">
              微信登录需要在 wechatbot-webhook 容器的登录页面扫码。点击下方按钮打开登录页面，使用手机微信扫描二维码完成登录。
            </p>
            <p className="text-xs text-amber-600 dark:text-amber-400">
              提示：基于 Web 微信协议，登录后约两天需要重新扫码。请确保微信账号已开通网页版登录权限。
            </p>
          </div>

          <div className="flex flex-col items-center gap-4">
            {loginUrl ? (
              <>
                <a href={loginUrl} target="_blank" rel="noopener noreferrer"
                  className="btn-primary py-2.5 px-6 flex items-center gap-2 text-sm">
                  <ExternalLink size={16} />打开微信扫码登录页面
                </a>
                <div className="w-full">
                  <iframe src={loginUrl} className="w-full h-96 rounded-lg border border-gray-200 dark:border-gray-700 bg-white"
                    title="WeChat Login" />
                </div>
              </>
            ) : (
              <div className="text-center space-y-2">
                <Loader2 size={32} className="animate-spin text-gray-400 mx-auto" />
                <p className="text-sm text-gray-500">正在获取登录地址...</p>
                <p className="text-xs text-gray-400">请确保微信容器已启动</p>
              </div>
            )}
          </div>
        </div>
      )}

      <div className="card p-4 space-y-2">
        <h3 className="font-semibold text-sm">连接状态</h3>
        <div className="space-y-1 text-sm">
          <div className="flex justify-between">
            <span className="text-gray-500">微信容器</span>
            <span className={status?.connected ? 'text-emerald-600' : 'text-red-500'}>
              {status?.connected ? '已连接' : '未连接'}
            </span>
          </div>
          <div className="flex justify-between">
            <span className="text-gray-500">登录状态</span>
            <span className={isLoggedIn ? 'text-emerald-600' : 'text-amber-500'}>
              {isLoggedIn ? '已登录' : '未登录'}
            </span>
          </div>
        </div>
      </div>
    </div>
  );
}
