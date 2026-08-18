// 工作台 URL 状态总线（提案 1010 定）：URL search params 是面板间唯一通信渠道，
// 所有面板读写选中态都必须经过本模块，不得自行 useSearchParams 拼参数。
// 当前只有 path；sourceType/sourceId、left/right 双选参数由 1011/1012 引入，
// 届时在此扩展并保持命名与后端 API query 参数一致。

export type WorkbenchParams = {
  path: string;
};

export function readWorkbenchParams(params: URLSearchParams): WorkbenchParams {
  return { path: params.get('path') ?? '' };
}

export function writePathParam(params: URLSearchParams, path: string) {
  params.set('path', path);
}
