// graph 布局测试（自 Go 版 graph_topology_test.go 移植；建仓脚本改为合成的
// sha/parents 列表，不再依赖真实 git）。
//
// 不比对 golden 值（拓扑组合太多），而是校验可证明的不变量——任何一个被破坏
// 都直接定位到「布局算错了」还是渲染问题：
//
//	不变量 A（边连通）：每条 child→parent 边，从 child 节点出发存在连续线段链
//	  到达 parent 节点；首段从 child 泳道出发，末段落到 parent 泳道。
//	不变量 B（无悬空线）：每段线的起点要么是本行节点的泳道（节点出线），
//	  要么承接上一段线的终点；终点要么是下一行节点的泳道，要么有下一段延续。
//	不变量 C（节点占位）：节点泳道互不冲突地落在各自行的线图上（首段 From 覆盖）。

import { describe, expect, it } from 'vitest';

import { computeGraph, simplifyLite, type GraphWire } from './graph-layout';

interface Entry {
  sha: string;
  parents: string[];
}

// --- 合成拓扑（新→旧的拓扑序；sha 用可读代号） ---

const linear: Entry[] = [
  { sha: 'c3', parents: ['c2'] },
  { sha: 'c2', parents: ['c1'] },
  { sha: 'c1', parents: [] },
];

// main: c1←m1←merge；feature: c1←f1←f2（merge 双父）
const branchMerge: Entry[] = [
  { sha: 'merge', parents: ['m1', 'f2'] },
  { sha: 'm1', parents: ['c1'] },
  { sha: 'f2', parents: ['f1'] },
  { sha: 'f1', parents: ['c1'] },
  { sha: 'c1', parents: [] },
];

const twoUnmerged: Entry[] = [
  { sha: 'm1', parents: ['c1'] },
  { sha: 'b2c', parents: ['c1'] },
  { sha: 'b1c', parents: ['c1'] },
  { sha: 'c1', parents: [] },
];

// 章鱼合并（3 父）
const octopus: Entry[] = [
  { sha: 'oct', parents: ['m1', 'b1c', 'b2c'] },
  { sha: 'm1', parents: ['c1'] },
  { sha: 'b2c', parents: ['c1'] },
  { sha: 'b1c', parents: ['c1'] },
  { sha: 'c1', parents: [] },
];

// 连续两次合并
const twoMerges: Entry[] = [
  { sha: 'merge2', parents: ['merge1', 'b2c'] },
  { sha: 'merge1', parents: ['m1', 'b1c'] },
  { sha: 'b2c', parents: ['m1'] },
  { sha: 'm1', parents: ['c1'] },
  { sha: 'b1c', parents: ['c1'] },
  { sha: 'c1', parents: [] },
];

// 嵌套分支：main ← merge-outer ←(outer 末提交, inner 分支)；outer 内部又合并 inner
const nested: Entry[] = [
  { sha: 'mergeOuter', parents: ['m1', 'o2m'] },
  { sha: 'm1', parents: ['c1'] },
  { sha: 'o2m', parents: ['o2', 'i1'] }, // outer 上 merge inner
  { sha: 'o2', parents: ['o1'] },
  { sha: 'i1', parents: ['o1'] },
  { sha: 'o1', parents: ['c1'] },
  { sha: 'c1', parents: [] },
];

// 晚切支线：dev 从 c1 分出但 dev 首提交排在主线下一提交之后 →
// dev 节点须让位给主线（defer 逻辑的回归场景）
const lateBranch: Entry[] = [
  { sha: 'merge', parents: ['m1', 'd1'] },
  { sha: 'm1', parents: ['c1'] },
  { sha: 'd1', parents: ['c1'] },
  { sha: 'c1', parents: [] },
];

// --- 不变量校验（自 Go 版 verifyGraphInvariants 移植） ---

function verifyGraphInvariants(commits: Entry[], laid: { sha: string; lane: number }[], wires: GraphWire[]) {
  expect(laid, '节点数量应与 commit 数一致').toHaveLength(commits.length);
  const nodeBySha = new Map<string, number>(); // sha → row
  laid.forEach((n, i) => {
    expect(n.lane, `节点 ${n.sha} lane 非法`).toBeGreaterThanOrEqual(0);
    nodeBySha.set(n.sha, i);
  });
  const wiresByRow = new Map<number, GraphWire[]>();
  for (const w of wires) {
    expect(w.from, `线段泳道非法 ${JSON.stringify(w)}`).toBeGreaterThanOrEqual(0);
    expect(w.to, `线段泳道非法 ${JSON.stringify(w)}`).toBeGreaterThanOrEqual(0);
    expect(w.row, `线段行号越界 ${JSON.stringify(w)}`).toBeLessThan(laid.length - 1);
    const list = wiresByRow.get(w.row) ?? [];
    list.push(w);
    wiresByRow.set(w.row, list);
  }

  // 不变量 A：每条 child→parent 边存在连续线段链。
  // 分叉点上同一泳道有多条 From 相同的线段（各父各一条），不能贪心逐段选——
  // 用 DFS 找「一条不重用线段、从子节点泳道出发、末段落到父节点泳道」的路径。
  const findPath = (from: number, band: number, endBand: number, endLane: number, used: Set<string>): boolean => {
    if (band > endBand) return false;
    const segs = wiresByRow.get(band) ?? [];
    for (let idx = 0; idx < segs.length; idx++) {
      const w = segs[idx];
      const key = `${band}:${idx}`;
      if (used.has(key) || w.from !== from) continue;
      if (band === endBand) {
        if (w.to === endLane) {
          used.add(key);
          return true;
        }
        continue;
      }
      used.add(key);
      if (findPath(w.to, band + 1, endBand, endLane, used)) return true;
      used.delete(key);
    }
    return false;
  };
  commits.forEach((c, i) => {
    for (const p of c.parents) {
      const pRow = nodeBySha.get(p);
      expect(pRow, `父提交 ${p} 不在节点列表`).toBeDefined();
      const used = new Set<string>();
      const ok = findPath(laid[i].lane, i, pRow! - 1, laid[pRow!].lane, used);
      expect(ok, `边 ${c.sha}→${p} 无连续线段路径`).toBe(true);
    }
  });

  // 不变量 B：无悬空线段
  for (const [row, segs] of wiresByRow) {
    for (const w of segs) {
      // 起点：本行节点出线，或承接上一段
      let fromOk = w.from === laid[row].lane;
      if (!fromOk) {
        fromOk = (wiresByRow.get(row - 1) ?? []).some((prev) => prev.to === w.from);
      }
      expect(fromOk, `悬空起点: band ${row} ${JSON.stringify(w)}`).toBe(true);
      // 终点：下一行节点入线，或有下段延续
      let toOk = w.to === laid[row + 1].lane;
      if (!toOk && row + 1 <= laid.length - 2) {
        toOk = (wiresByRow.get(row + 1) ?? []).some((next) => next.from === w.to);
      }
      expect(toOk, `悬空终点: band ${row} ${JSON.stringify(w)}`).toBe(true);
    }
  }
}

// --- 用例 ---

describe.each([
  ['线性历史', linear],
  ['分支+合并', branchMerge],
  ['双分支未合并', twoUnmerged],
  ['章鱼合并（3 父）', octopus],
  ['连续两次合并', twoMerges],
  ['嵌套分支合并', nested],
  ['晚切支线（让位）', lateBranch],
])('computeGraph 拓扑不变量：%s', (_name, commits) => {
  it('边连通 / 无悬空线 / 节点占位', () => {
    const { nodes, wires } = computeGraph(commits);
    verifyGraphInvariants(commits, nodes, wires);
  });

  it('确定性：同输入两次计算结果逐字段一致', () => {
    const a = computeGraph(commits);
    const b = computeGraph(commits);
    expect(b.nodes).toEqual(a.nodes);
    expect(b.wires).toEqual(a.wires);
  });
});

describe('虚拟节点（dirty worktree 合成提交）', () => {
  // 基线：分支+合并拓扑
  // 顶部插入一个虚拟节点（parents 指向主线 tip m1），模拟 dirty 的主工作副本
  it('挂在 tip 上方，不扰动下方既有节点的泳道与颜色', () => {
    const base = computeGraph(branchMerge);
    const virtual = [{ sha: 'worktree:main', parents: ['merge'] }, ...branchMerge];
    const withVt = computeGraph(virtual);

    // 下方每个真实节点的 lane/color 与基线完全一致（顶部纯插入）
    for (let i = 0; i < branchMerge.length; i++) {
      expect(withVt.nodes[i + 1].lane).toBe(base.nodes[i].lane);
      expect(withVt.nodes[i + 1].color).toBe(base.nodes[i].color);
    }
    // 虚拟节点本身出现在第 0 行
    expect(withVt.nodes[0].sha).toBe('worktree:main');
    verifyGraphInvariants(virtual, withVt.nodes, withVt.wires);
  });

  it('多个虚拟节点按输入顺序占位，且全部边连通', () => {
    const virtual = [
      { sha: 'worktree:main', parents: ['merge'] },
      { sha: 'worktree:dev', parents: ['f2'] }, // dev 副本挂在 feature tip 上
      ...branchMerge,
    ];
    const { nodes, wires } = computeGraph(virtual);
    verifyGraphInvariants(virtual, nodes, wires);
    expect(nodes[0].sha).toBe('worktree:main');
    expect(nodes[1].sha).toBe('worktree:dev');
  });

  it('泳道号复用不串灰线：clean worktree tip 复用虚拟节点让出的泳道，其出线不属于虚拟线', () => {
    // 真实场景（develop 仓库现场）：两个 dirty 副本的虚拟节点在顶部，
    // 第二个虚拟节点因让位（defer）把泳道 1 留洞，紧随其后的真实提交
    // （另一 clean worktree 的 tip，parent = 主线 tip）复用泳道 1。
    // 旧实现按泳道号判灰线，该真实提交的出线被误置灰——线身份（origin）须与泳道号解耦
    const list = [
      { sha: 'worktree:main', parents: ['dev'] }, // 虚拟 1：主线 dirty
      { sha: 'worktree:tpl', parents: ['tpl'] }, // 虚拟 2：让位，泳道 1 留洞
      { sha: 'wt01', parents: ['dev'] }, // clean worktree tip：复用泳道 1
      { sha: 'dev', parents: ['tpl'] },
      { sha: 'tpl', parents: [] },
    ];
    const { nodes, wires } = computeGraph(list);
    verifyGraphInvariants(list, nodes, wires);
    const rowOf = new Map(nodes.map((n, i) => [n.sha, i]));
    // wt01 节点出线：origin 是 wt01 自己的行（真实提交），不是任何虚拟节点行
    const out = wires.find((w) => w.row === rowOf.get('wt01')! && w.from === nodes[rowOf.get('wt01')!].lane)!;
    expect(out.origin).toBe(rowOf.get('wt01'));
    // 虚拟节点自己的出线 origin 即其所在行
    for (const v of ['worktree:main', 'worktree:tpl']) {
      const r = rowOf.get(v)!;
      const w = wires.find((x) => x.row === r && x.from === nodes[r].lane)!;
      expect(w.origin).toBe(r);
    }
  });

  it('虚拟节点的父不在已持有数据中时，等待泳道延续到页底自然截止', () => {
    // 只加载了第一页（merge..m1），虚拟节点挂载的 m2 不在其中：
    // m2 泳道以穿越线延续到最后一行后截止——与分页底部的普通支线同语义
    const page = [
      { sha: 'worktree:main', parents: ['m2'] },
      { sha: 'merge', parents: ['m1', 'f2'] },
      { sha: 'm1', parents: ['c1'] },
    ];
    const { nodes, wires } = computeGraph(page);
    expect(nodes.map((n) => n.sha)).toEqual(['worktree:main', 'merge', 'm1']);
    // 线段集合确定性：虚拟节点出线（0 道）、m2 穿越线（0 道，页底截止）、
    // merge 的两条节点出线均从节点泳道出发（第二条拐向 f2 的等待泳道）
    expect(wires).toEqual([
      { row: 0, from: 0, to: 0, color: 1, origin: 0 }, // worktree:main → 等待中的 m2 泳道
      { row: 1, from: 0, to: 0, color: 1, origin: 0 }, // m2 泳道穿越（父未加载，延续到页底）
      { row: 1, from: 1, to: 1, color: 2, origin: 1 }, // merge → m1（首父接管原泳道）
      { row: 1, from: 1, to: 2, color: 3, origin: 1 }, // merge → f2（额外父拐弯曲线）
    ]);
  });
});

// --- simplifyLite（轻量模式） ---

interface LiteEntry {
  sha: string;
  parents: string[];
  refs?: string[];
}

describe('simplifyLite', () => {
  it('线性链只留带 ref 的端点，且边重接到保留祖先', () => {
    const list: LiteEntry[] = [
      { sha: 'tip', parents: ['b'], refs: ['main'] },
      { sha: 'b', parents: ['a'] },
      { sha: 'a', parents: ['root'] },
      { sha: 'root', parents: [] },
    ];
    const out = simplifyLite(list);
    expect(out.map((c) => c.sha)).toEqual(['tip', 'root']);
    expect(out[0].parents).toEqual(['root']);
  });

  it('merge 提交与分叉点保留，中间线性节点隐藏', () => {
    // main: root←m1←m2←merge；feature: root←f1←f2（merge 双父）
    const list: LiteEntry[] = [
      { sha: 'merge', parents: ['m2', 'f2'], refs: ['main'] },
      { sha: 'm2', parents: ['m1'] },
      { sha: 'm1', parents: ['root'] },
      { sha: 'f2', parents: ['f1'] },
      { sha: 'f1', parents: ['root'] },
      { sha: 'root', parents: [] },
    ];
    const out = simplifyLite(list);
    // root 是分叉点（children=2）；f2 链上无 ref，f1/f2 都隐藏
    expect(out.map((c) => c.sha)).toEqual(['merge', 'root']);
    expect(out[0].parents).toEqual(['root']);
  });

  it('分支 tip 带 ref 时保留，边沿各自链重接', () => {
    const list: LiteEntry[] = [
      { sha: 'tip', parents: ['a'], refs: ['main'] },
      { sha: 'a', parents: ['fork'] },
      { sha: 'b2', parents: ['b1'], refs: ['dev'] },
      { sha: 'b1', parents: ['fork'] },
      { sha: 'fork', parents: ['root'] },
      { sha: 'root', parents: [] },
    ];
    const out = simplifyLite(list);
    expect(out.map((c) => c.sha)).toEqual(['tip', 'b2', 'fork', 'root']);
    expect(out[0].parents).toEqual(['fork']);
    expect(out[1].parents).toEqual(['fork']);
    expect(out[2].parents).toEqual(['root']);
  });

  it('tag 徽标也算 ref，tagged 提交保留', () => {
    const list: LiteEntry[] = [
      { sha: 'head', parents: ['t'], refs: ['main'] },
      { sha: 't', parents: ['root'], refs: ['v1.0'] },
      { sha: 'root', parents: [] },
    ];
    const out = simplifyLite(list);
    expect(out.map((c) => c.sha)).toEqual(['head', 't', 'root']);
  });

  it('两条隐藏链汇到同一保留祖先时去重（已知退化：merge 塌成单父）', () => {
    const list: LiteEntry[] = [
      { sha: 'merge', parents: ['m', 'f'] },
      { sha: 'm', parents: ['root'] },
      { sha: 'f', parents: ['root'] },
      { sha: 'root', parents: [] },
    ];
    const out = simplifyLite(list);
    expect(out.map((c) => c.sha)).toEqual(['merge', 'root']);
    expect(out[0].parents).toEqual(['root']);
  });

  it('keepRecent：顶部最近 N 个无条件保留', () => {
    const list: LiteEntry[] = [
      { sha: 'r5', parents: ['r4'] },
      { sha: 'r4', parents: ['r3'] },
      { sha: 'r3', parents: ['r2'] },
      { sha: 'r2', parents: ['r1'] },
      { sha: 'r1', parents: ['tip'], refs: ['main'] },
      { sha: 'tip', parents: ['root'] },
      { sha: 'root', parents: [] },
    ];
    const out = simplifyLite(list, 5);
    expect(out.map((c) => c.sha)).toEqual(['r5', 'r4', 'r3', 'r2', 'r1', 'root']);
    expect(out[0].parents).toEqual(['r4']);
    expect(out[4].parents).toEqual(['root']); // r1 的边越过隐藏的 tip 重接到 root
  });

  it('父提交超出已加载范围（链断裂）时丢弃该边', () => {
    const list: LiteEntry[] = [
      { sha: 'tip', parents: ['a'], refs: ['main'] },
      { sha: 'a', parents: ['unloaded'] },
    ];
    const out = simplifyLite(list);
    expect(out.map((c) => c.sha)).toEqual(['tip']);
    expect(out[0].parents).toEqual([]);
  });
});
