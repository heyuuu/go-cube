// 工作台 URL 状态总线（提案 1010 定）：URL search params 是面板间唯一通信渠道，
// 所有面板读写选中态都必须经过本模块，不得自行 useSearchParams 拼参数。
// 命名与后端 API query 参数保持一致：sourceType/sourceId、leftType/leftId、rightType/rightId。

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

function readSource(params: URLSearchParams, prefix: string): TreeSource | null {
  const type = params.get(`${prefix}Type`);
  const id = params.get(`${prefix}Id`);
  if (!type || !id) return null;
  if (type !== 'commit' && type !== 'ref' && type !== 'worktree') return null;
  return { type, id };
}

function writeSource(params: URLSearchParams, prefix: string, src: TreeSource | null) {
  if (src) {
    params.set(`${prefix}Type`, src.type);
    params.set(`${prefix}Id`, src.id);
  } else {
    params.delete(`${prefix}Type`);
    params.delete(`${prefix}Id`);
  }
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
  params.delete('sourceType');
  params.delete('sourceId');
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
  return !!a && !!b && a.type === b.type && a.id === b.id;
}

export function sourceLabel(src: TreeSource | null): string {
  if (!src) return '';
  if (src.type === 'commit') return `${src.id.slice(0, 7)}`;
  if (src.type === 'ref') return src.id;
  return src.id.split('/').pop() || src.id;
}
