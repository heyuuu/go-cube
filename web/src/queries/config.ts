import { useQuery } from '@tanstack/react-query';

import { apiGet } from '@/api/client';

export function useConfig() {
  return useQuery({ queryKey: ['config'], queryFn: () => apiGet('/api/config') });
}
