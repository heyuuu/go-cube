import { apiGet, apiPost } from './client';
import type { ProjectListResult } from './client';
// 类型契约测试：固化 apiGet/apiPost 调用点的推导精度，tsgo 编译即断言。
// 「ts-expect-error 指令」成对出现——若某处推导退化（any / 跨端点 union），
// 对应的错误不再触发，tsgo 会因「未使用的指令」而报错，契约破裂可见。
import type { components } from './schema';

export async function typeContractProbe() {
  // GET 返回类型应精确为 ProjectListResult（而非所有端点响应的 union）
  const res = await apiGet('/api/project/list');
  const ok: ProjectListResult = res;
  // @ts-expect-error union 退化时 Config 也在 union 里，此赋值会通过 → 契约破裂
  const wrong: components['schemas']['Config'] = res;
  void ok;
  void wrong;

  // 路径约束：必须是真实存在的 GET 端点
  // @ts-expect-error /api/opener/open 是 POST 端点
  void apiGet('/api/opener/open');
  // @ts-expect-error 不存在的路径
  void apiGet('/api/not-exist');

  // GET 的 query 扁平直传：类型来自 op 的 parameters.query 声明
  await apiGet('/api/workbench/info', { path: '/x123' });
  // @ts-expect-error path 必填，漏传 query 报错
  await apiGet('/api/workbench/info');
  // @ts-expect-error query 字段名错误
  await apiGet('/api/workbench/info', { nope: 1 });
  // @ts-expect-error 未声明 query 的端点，传 query 报错
  await apiGet('/api/project/list', { name: 'x' });
  // 说明：「query 全可选可省」分支（原 /api/project/tree 探针）在端点移除后暂无真实
  // 端点可测，QueryArg 类型仍保留该分支，待未来出现可选 query 端点时补回探针。

  // POST body 类型应精确为 OpenerOpenInputBody
  await apiPost('/api/opener/open', { path: '/tmp/x', opener: 'code' });
  // @ts-expect-error body 字段名错误
  await apiPost('/api/opener/open', { wrong: 'field' });
}
