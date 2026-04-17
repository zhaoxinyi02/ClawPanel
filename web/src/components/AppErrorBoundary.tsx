import React from 'react';

interface AppErrorBoundaryState {
  error: Error | null;
}

export default class AppErrorBoundary extends React.Component<React.PropsWithChildren, AppErrorBoundaryState> {
  state: AppErrorBoundaryState = { error: null };

  static getDerivedStateFromError(error: Error): AppErrorBoundaryState {
    return { error };
  }

  componentDidCatch(error: Error, info: React.ErrorInfo) {
    console.error('App render error:', error, info);
  }

  render() {
    if (this.state.error) {
      return (
        <div className="min-h-screen bg-slate-50 px-6 py-10 text-slate-900 dark:bg-slate-950 dark:text-slate-50">
          <div className="mx-auto max-w-3xl rounded-3xl border border-red-200/80 bg-white/95 p-6 shadow-[0_20px_60px_rgba(15,23,42,0.08)] dark:border-red-900/40 dark:bg-slate-900/95">
            <div className="text-sm font-semibold uppercase tracking-[0.18em] text-red-500">Frontend Error</div>
            <h1 className="mt-3 text-2xl font-bold">页面渲染失败</h1>
            <p className="mt-2 text-sm leading-6 text-slate-600 dark:text-slate-300">
              前端捕获到了未处理异常。请把下面这段错误信息发给我，我会继续直接修。
            </p>
            <pre className="mt-5 overflow-auto rounded-2xl bg-slate-950 px-4 py-4 text-xs leading-6 text-red-200">
              {this.state.error.stack || this.state.error.message}
            </pre>
            <button
              onClick={() => window.location.reload()}
              className="mt-5 rounded-xl bg-blue-600 px-4 py-2 text-sm font-semibold text-white transition-colors hover:bg-blue-700"
            >
              重新加载页面
            </button>
          </div>
        </div>
      );
    }

    return this.props.children;
  }
}
