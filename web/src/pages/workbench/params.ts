// 工作台 URL 状态总线（提案 1010 定）：URL search params 是面板间唯一通信渠道，
// 所有面板读写选中态都必须经过本模块，不得自行 useSearchParams 拼参数。
// 选中目标统一序列化为 "type://id"（commit://<sha>、ref://refs/heads/master、worktree://<目录>），
// 参数名与后端 API 一致：source / left / right（文法见后端 workbench.ParseTreeSource）。

export type SourceType = 'commit' | 'ref' | 'worktree';

export type TreeSource = {
  type: SourceType;
  id: string;
};

export type WorkbenchParams = {
  path: string;
  source: TreeSource | null;
  left: TreeSource | null;
  right: TreeSource | null;
};

// 解析 "type://id"：已知 scheme 精确前缀匹配，余部原样取出（与后端解析同构）。
// 不做通用 URL 解析——worktree 路径自身含 "://" 也不歧义（scheme 在第一个 "://" 前已确定）。
export function parseSource(s: string | null): TreeSource | null {
  for (const type of ['commit', 'ref', 'worktree'] as const) {
    if (s?.startsWith(type + '://')) {
      const id = s.slice(type.length + 3);
      return id === '' ? null : { type, id };
    }
  }
  return null;
}

export function toUri(src: TreeSource): string {
  return `${src.type}://${src.id}`;
}

// 剥规范全名前缀得短名（refs/heads/master → master）；非全名形态（HEAD、短名手输）原样返回。
// 展示与 commit 图 decorate 徽标（短名）的比对共用。
export function refShortName(id: string): string {
  for (const p of ['refs/heads/', 'refs/tags/', 'refs/remotes/']) {
    if (id.startsWith(p)) return id.slice(p.length);
  }
  return id;
}

function readSource(params: URLSearchParams, key: 'source' | 'left' | 'right'): TreeSource | null {
  return parseSource(params.get(key));
}

function writeSource(params: URLSearchParams, key: 'source' | 'left' | 'right', src: TreeSource | null) {
  if (src) params.set(key, toUri(src));
  else params.delete(key);
}

export function readWorkbenchParams(params: URLSearchParams): WorkbenchParams {
  return {
    path: params.get('path') ?? '',
    source: readSource(params, 'source'),
    left: readSource(params, 'left'),
    right: readSource(params, 'right'),
  };
}

export function writePathParam(params: URLSearchParams, path: string) {
  params.set('path', path);
}

// 单选：清空双选，只留 source
export function selectSource(params: URLSearchParams, src: TreeSource) {
  writeSource(params, 'left', null);
  writeSource(params, 'right', null);
  writeSource(params, 'source', src);
}

// 双选追加（cmd/ctrl 点第二个目标）：left 空则填 left，right 空且不同才填 right，
// 两边都满则重开一轮（新目标为 left）
export function selectDiffSide(params: URLSearchParams, src: TreeSource) {
  const cur = readWorkbenchParams(params);
  params.delete('source');
  if (!cur.left || (cur.left && cur.right)) {
    writeSource(params, 'left', src);
    writeSource(params, 'right', null);
    return;
  }
  if (sameSource(cur.left, src)) return;
  writeSource(params, 'right', src);
}

export function clearSelection(params: URLSearchParams) {
  writeSource(params, 'source', null);
  writeSource(params, 'left', null);
  writeSource(params, 'right', null);
}

export function sameSource(a: TreeSource | null, b: TreeSource | null): boolean {
  return !!a && !!b && toUri(a) === toUri(b);
}

export function sourceLabel(src: TreeSource | null): string {
  if (!src) return '';
  if (src.type === 'commit') return src.id.slice(0, 7);
  if (src.type === 'ref') return refShortName(src.id);
  return src.id.split('/').pop() || src.id;
}
