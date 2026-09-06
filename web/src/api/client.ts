import createClient, { type ClientPathsWithMethod, type MethodResponse } from 'openapi-fetch';

import type { components, paths } from './schema';

// --- 类型（schema 直取，gen:api 后随契约自动更新） ---

export type Config = components['schemas']['Config'];
export type Opener = components['schemas']['OpenerDTO'];
export type Project = components['schemas']['ProjectDTO'];
export type ScanRule = components['schemas']['ScanRule'];
export type CloneRule = components['schemas']['CloneRule'];
export type Forge = components['schemas']['Forge'];
export type ForgeAccount = components['schemas']['Account'];
export type ForgeNamespace = components['schemas']['Namespace'];
export type ReconcileResult = components['schemas']['ReconcileResult'];
export type ProjectInfoResult = components['schemas']['ProjectInfoResult'];
export type ProjectListResult = components['schemas']['ProjectListResult'];

export const apiClient = createClient<paths>();

// 后端统一 ApiOutput{ok,message,data}：ok=false / HTTP 错误 / 非 JSON 一律 throw
// （React Query error 分支），业务调用点只拿 data。全工程唯一的手写胶水。

interface Envelope {
  ok: boolean;
  message: string;
  data: unknown;
}

export async function unwrap<T extends Envelope>(
  req: Promise<{ data?: T; error?: unknown; response: Response }>,
): Promise<T['data']> {
  const { data, error, response } = await req;
  if (error) {
    const err = error as { detail?: string; title?: string };
    throw new Error(err.detail || err.title || `请求失败（HTTP ${response.status}）`);
  }
  if (!data) throw new Error(`响应为空或非 JSON（HTTP ${response.status}）`);
  if (!data.ok) throw new Error(data.message || '请求失败');
  return data.data;
}

// 动态分发层：泛型 P 把「路径字面量 → 响应类型」的推导保留到调用点（官方 ClientPathsWithMethod /
// MethodResponse 工具类型），不再退化成所有端点的 union。
//   const list = await apiGet('/api/project/list');            // list: ProjectListResult
//   await apiGet('/api/project/info', { path });              // query 扁平直传
//   await apiPost('/api/opener/open', { path, app });          // body 类型来自 OpenerOpenInputBody

type GetPaths = ClientPathsWithMethod<typeof apiClient, 'get'>;
type PostPaths = ClientPathsWithMethod<typeof apiClient, 'post'>;

// GET 的 query 类型：来自 op 的 parameters.query 声明；端点未声明时为
// undefined（query 参数传了就编译报错）。包装层内部转成 openapi-fetch 的
// { params: { query } } 形态，调用点不感知嵌套。
type GetQuery<P extends GetPaths> = NonNullable<NonNullable<NonNullable<paths[P]['get']>['parameters']>['query']>;

// POST 请求体类型：从 op 的 requestBody 提取（NonNullable 剥掉可选层）
type PostBody<P extends PostPaths> = NonNullable<
  NonNullable<paths[P]['post']>['requestBody']
>['content']['application/json'];

// query 形态判别（元组包裹阻断条件类型的 never 分配）：
// 无参数端点 → 禁止传；参数全可选 → 可省；含必填参数 → 必传
type QueryArg<Q> = [Q] extends [undefined] ? [query?: undefined] : [{}] extends [Q] ? [query?: Q] : [query: Q];

export async function apiGet<P extends GetPaths>(path: P, ...query: QueryArg<GetQuery<P>>) {
  return unwrap(getRaw(path, query[0]));
}

export async function apiPost<P extends PostPaths>(path: P, body: PostBody<P>) {
  return unwrap(postRaw(path, body));
}

// --- 内部分发桥 ---
// openapi-fetch 的类型依赖「字面量直接调用」形态；在泛型 P 上下文里，
// ParseAs/Init 等条件类型保持延迟（TS 认为 data 可能是 string/Blob/null），
// 无法自行证明符合 envelope 形状。桥内做一次受控断言收敛类型：
// 运行时恒为 JSON envelope（unwrap 运行时会再校验 ok/data 字段），
// 桥两端类型各自精确——入口 P 泛型约束、出口 MethodResponse 推导。

function getRaw<P extends GetPaths>(path: P, query?: GetQuery<P>) {
  const get = apiClient.GET as (
    path: never,
    init?: { params: { query: GetQuery<P> } },
  ) => Promise<{
    data?: Envelope & MethodResponse<typeof apiClient, 'get', P>;
    error?: unknown;
    response: Response;
  }>;
  // 扁平 query → openapi-fetch 的嵌套形态，调用点不感知
  return get(path as never, query === undefined ? undefined : { params: { query } });
}

function postRaw<P extends PostPaths>(path: P, body: PostBody<P>) {
  const post = apiClient.POST as (
    path: never,
    init: { body: never },
  ) => Promise<{
    data?: Envelope & MethodResponse<typeof apiClient, 'post', P>;
    error?: unknown;
    response: Response;
  }>;
  return post(path as never, { body: body as never });
}
