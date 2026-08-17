import { useQuery } from '@tanstack/react-query';

import { apiGet } from '@/api/client';

export function useMdList(path: string) {
  return useQuery({
    queryKey: ['md', 'list', path],
    queryFn: () => apiGet('/api/md/list', { path }),
    enabled: path !== '',
  });
}

export function useMdContent(path: string) {
  return useQuery({
    queryKey: ['md', 'content', path],
    queryFn: () => apiGet('/api/md/content', { path }),
    enabled: path !== '',
    staleTime: 0, // 本地文件，重进页面即重新读
  });
}
