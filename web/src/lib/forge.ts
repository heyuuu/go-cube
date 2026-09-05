// forge 前端匹配：从 repo remote URL 解析 host，供展示侧查已配置 forge（提案 1040）。
// 语义与后端 forge.RepoHost 对齐（git@host:path / scheme://[user@]host[:port]/path 两形态），
// 无法解析返回空串（按未匹配处理）。host 匹配前统一小写、去尾部点号（后端 NormalizeHost 同规则）。
export function repoHostOf(raw: string | null | undefined): string {
  const s = (raw ?? '').trim();
  if (!s) return '';
  if (s.startsWith('git@')) {
    return s.slice(4).split(':')[0].toLowerCase().replace(/\.$/, '');
  }
  const m = /^[a-zA-Z][a-zA-Z0-9+.-]*:\/\/(?:[^/@]+@)?([^/?#]+)/.exec(s);
  return m ? m[1].toLowerCase().replace(/\.$/, '') : '';
}

// forge 筛选谓词（单选，与页面 git 筛选同语义层级）：
//   - 'all'：全部；
//   - 具体 forge host：只看「本 repo 的 host 是否等于该 forge」，不涉及其它 repo / 其它 forge 的匹配情况；
//   - 'other'：有 remote 但 host 未命中任何已配置 forge（含无法解析的 URL）；
//   - 'none'：无 remote（含未采集 gitInfo）。
export function matchForgeFilter(
  repoUrl: string | null | undefined,
  filter: string,
  forgeHosts: readonly string[],
): boolean {
  if (filter === 'all') return true;
  if (filter === 'none') return !repoUrl;
  const host = repoHostOf(repoUrl);
  if (filter === 'other') return !!repoUrl && !forgeHosts.includes(host);
  return host === filter;
}
