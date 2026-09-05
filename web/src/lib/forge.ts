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
