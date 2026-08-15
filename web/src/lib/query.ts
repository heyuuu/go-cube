import { QueryClient } from '@tanstack/react-query';

export const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      // 数据源是 server 侧 gitcache 快照（5 分钟级变化），前端 30s 轮询；
      // staleTime 对齐轮询周期，窗口切换/组件重挂载时不重复请求
      staleTime: 30_000,
      // 本地 server，失败快速暴露比重试掩盖问题更有价值
      retry: 1,
    },
  },
});
