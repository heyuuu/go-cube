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

// 项目 git 快照中参与 forge 匹配的 remote 形态（ProjectDTO.gitInfo 的子结构）。
interface GitRemoteLike {
  name?: string;
  url: string;
}

interface GitInfoLike {
  repoUrl?: string | null;
  remotes?: GitRemoteLike[] | null;
}

// 项目的全部 forge host（1042 修：多 remote 项目同时匹配多个 forge）——
// repoUrl（origin）在前，其余 remotes 按序去重；无法解析的 URL 产生空串由调用方忽略。
export function repoHostsOf(gitInfo: GitInfoLike | null | undefined): string[] {
  if (!gitInfo) return [];
  const urls = [gitInfo.repoUrl, ...(gitInfo.remotes ?? []).map((r) => r.url)];
  const hosts: string[] = [];
  for (const u of urls) {
    const h = repoHostOf(u);
    if (h && !hosts.includes(h)) hosts.push(h);
  }
  return hosts;
}

// forge 筛选谓词（单选，入参为项目全部 remote host，语义与单 remote 版一致）：
//   - 'all'：全部；
//   - 具体 forge host：项目任一 remote host 等于该 forge；
//   - 'other'：有 remote 但没有任何 host 命中已配置 forge（含无法解析的 URL）；
//   - 'none'：无 remote（含未采集 gitInfo）。
export function matchForgeFilter(hosts: readonly string[], filter: string, forgeHosts: readonly string[]): boolean {
  if (filter === 'all') return true;
  if (filter === 'none') return hosts.length === 0;
  if (filter === 'other') return hosts.length > 0 && !hosts.some((h) => forgeHosts.includes(h));
  return hosts.includes(filter);
}

// 从 repo remote URL 解析「namespace/name」展示形态（去 scheme/host 与 .git 尾缀），
// 无法解析返回空串。RepoUrl 列的展示文本用它（host 由 forge icon 承载，不重复显示）。
export function repoPathOf(raw: string | null | undefined): string {
  const host = repoHostOf(raw);
  const s = (raw ?? '').trim();
  if (!s || !host) return '';
  const path = s.startsWith('git@')
    ? s.slice(4 + host.length + 1)
    : s.replace(/^[a-zA-Z][a-zA-Z0-9+.-]*:\/\/[^/]+\//, '');
  return path.replace(/\.git$/, '').replace(/\/$/, '');
}
