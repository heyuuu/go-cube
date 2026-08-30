// commit graph 的 lane 布局（自 Go 版 computeGraph 移植，提案 1011 的拓扑升级；
// 布局在前端对「已持有数据」计算，接口只提供纯日志列表）。
//
// 算法：经典 active-lanes 扫描（pvigier / DoltHub 博客的做法）。
// 按拓扑序遍历 commit（上游用 --topo-order 保证）：
//   - lanes 是「等待出现的 commit sha」占位列表，索引即泳道；
//   - 当前 commit 出现在某泳道 → 节点画在该泳道并腾出位置；
//     不在任何泳道（根提交等）→ 复用最靠左的空洞，无洞则右侧新开；
//   - 首父占据原泳道（直线延续、继承泳道色）；首父已被其它子提交占位则
//     合并过去、原泳道留洞；其余父各占新位（合并曲线的来源）；
//   - **关键：泳道不即时压缩**——空位留洞，只裁剪尾部连续空洞。
//     即时删除会让右侧泳道整体左移，支线被迫在错误的行提前拐弯
//     （叉出/汇入位置偏一行）；留洞保证位置稳定，拐弯只出现在
//     真正 fork/merge 的段上。
// 连线推导（第二遍，纯查表）：第 i 行到 i+1 行的线段 = lanesAfter[i] 的每个
// 条目映射到 lanesAfter[i+1] 中的新位置；条目正是 i+1 行的 commit 时映射到
// 该节点的泳道（线进入节点）。分段表达使渲染端无需建模跨行整线。

// 轻量模式简化（纯前端）：只保留「拓扑骨架」——带 ref 的提交（分支/tag/HEAD）、
// merge 提交、分叉点（children > 1）、根提交，以及最近 keepRecent 个提交——
// 其余线性节点隐藏；隐藏段的边重接到最近的保留祖先，保证分支切出/汇合的形状不断线。
// 已知退化：若 merge 的两条父链在被隐藏后汇到同一保留祖先（如两侧链上都没有
// 任何 ref），去重后 merge 会塌成单父直线——分支 tip 通常带 ref，实践中罕见。
export function simplifyLite<T extends { sha: string; parents?: string[] | null; refs?: readonly unknown[] | null }>(
  commits: readonly T[],
  keepRecent = 0,
): T[] {
  const bySha = new Map(commits.map((c) => [c.sha, c]));
  const childrenCount = new Map<string, number>();
  for (const c of commits) {
    for (const p of c.parents ?? []) childrenCount.set(p, (childrenCount.get(p) ?? 0) + 1);
  }
  const kept = new Set(
    commits
      .filter(
        (c, i) =>
          i < keepRecent || // 顶部最近 N 个无条件保留（渲染端在 N 边界画虚线分割）
          (c.parents ?? []).length === 0 || // 根提交 = 链的终点，保留
          (c.refs?.length ?? 0) > 0 ||
          (c.parents ?? []).length > 1 ||
          (childrenCount.get(c.sha) ?? 0) > 1,
      )
      .map((c) => c.sha),
  );

  // 被隐藏的都是线性节点（merge/分叉点必保留），沿首父链上溯即最近保留祖先
  const memo = new Map<string, string | null>();
  const nearestKept = (sha: string): string | null => {
    const hit = memo.get(sha);
    if (hit !== undefined) return hit;
    memo.set(sha, null); // 防环兜底
    let res: string | null = null;
    if (kept.has(sha)) res = sha;
    else {
      const p = bySha.get(sha)?.parents?.[0];
      res = p ? nearestKept(p) : null;
    }
    memo.set(sha, res);
    return res;
  };

  return commits
    .filter((c) => kept.has(c.sha))
    .map((c) => ({
      ...c,
      parents: [...new Set((c.parents ?? []).map(nearestKept).filter((x): x is string => x != null))],
    }));
}

export interface GraphWire {
  row: number; // 段的起始行（连到 row+1）
  from: number; // 起始泳道
  to: number; // 结束泳道
  color: number; // 调色板索引（渲染端映射具体颜色）
  origin: number; // 线的起始行（创建该等待泳道的节点行）：线的稳定身份，
  // 渲染端据此判定「虚拟节点的线」——泳道号会被复用（让位留洞、newLane 回填），
  // 不能作为灰线判定的键
}

export interface LaneInfo {
  lane: number;
  color: number;
}

interface graphLane {
  sha: string; // 空 sha = 空洞（已腾出待复用的泳道位）
  color: number;
  origin: number; // 创建该等待线的节点行（首父接管时沿线下传，见 GraphWire.origin）
}

// commits 须为拓扑序（新→旧）；扩展字段（refs/subject 等）原样带到返回节点。
// parents 允许 null（OpenAPI 对 Go 切片标 nullable），内部按空处理。
export function computeGraph<T extends { sha: string; parents?: string[] | null }>(commits: readonly T[]) {
  const nodes: (T & LaneInfo)[] = new Array(commits.length);
  // 每行处理完的 lanes 快照：既用于下一行定位，也用于第二遍连线推导
  const snapshots: graphLane[][] = new Array(commits.length);

  const lanes: graphLane[] = [];
  const laneIndexOf = (sha: string): number => {
    for (let i = 0; i < lanes.length; i++) if (lanes[i].sha === sha) return i;
    return -1;
  };
  // newLane：新支线复用最靠左的空洞，没有空洞才在右侧展开（控制总宽度）
  const newLane = (entry: graphLane): number => {
    for (let i = 0; i < lanes.length; i++) {
      if (lanes[i].sha === '') {
        lanes[i] = entry;
        return i;
      }
    }
    lanes.push(entry);
    return lanes.length - 1;
  };
  // trimTrailingHoles：裁掉尾部连续空洞（不移动任何占用泳道的位置）
  const trimTrailingHoles = () => {
    while (lanes.length > 0 && lanes[lanes.length - 1].sha === '') lanes.pop();
  };

  // fpChildren[p] = 把 p 作为首父的子提交行号列表。
  // 首父认领泳道时做「低泳道优先」：若本节点的首父还有其它首父子提交
  // 正等待在更靠左（更靠近主线）的泳道上，本节点不认领、让位给那条泳道，
  // 自身按合并处理——否则后切出的支线会把母分支的祖先链拖到右侧泳道，
  // 母分支反而画成拐弯的侧枝（嵌套分支场景的主视觉 bug）。
  const parentsOf = (c: { parents?: string[] | null }) => c.parents ?? [];
  const fpChildren = new Map<string, number[]>();
  commits.forEach((c, i) => {
    const ps = parentsOf(c);
    if (ps.length > 0) {
      const list = fpChildren.get(ps[0]) ?? [];
      list.push(i);
      fpChildren.set(ps[0], list);
    }
  });

  const deferred = new Map<number, number>(); // 行号 → 让位目标泳道（出线拐弯的落点）
  const nodeOrigins = new Array<number>(commits.length); // 行号 → 该节点出线的线身份（进入该节点的线的起始行）
  let colorSeq = 0;
  commits.forEach((c, i) => {
    let lane = laneIndexOf(c.sha);
    if (lane < 0) {
      colorSeq++;
      lane = newLane({ sha: c.sha, color: colorSeq, origin: i });
    }
    const color = lanes[lane].color;
    // 节点出线延续进入本节点的线（未在等待泳道中出现 = 新线的起点，origin 即本行）
    const lineOrigin = lanes[lane].origin;
    nodeOrigins[i] = lineOrigin;
    nodes[i] = { ...c, lane, color };

    const ps = parentsOf(c);
    if (ps.length > 0) {
      // 让位判定：存在更靠左的首父兄弟等待条目 → 让出本泳道，出线拐向兄弟泳道
      let deferTo = -1;
      for (const j of fpChildren.get(ps[0]) ?? []) {
        if (j === i) continue;
        const jl = laneIndexOf(commits[j].sha);
        if (jl >= 0 && jl < lane && (deferTo < 0 || jl < deferTo)) deferTo = jl;
      }
      if (deferTo >= 0 || laneIndexOf(ps[0]) >= 0) {
        // 首父已被其它子提交占位（或让位）→ 合并过去，原泳道留洞（不左移）
        lanes[lane] = { sha: '', color: 0, origin: 0 };
        if (deferTo >= 0) deferred.set(i, deferTo);
      } else {
        lanes[lane] = { sha: ps[0], color, origin: lineOrigin };
      }
      // 其余父各占新位（若未占位）
      for (const p of ps.slice(1)) {
        if (laneIndexOf(p) < 0) {
          colorSeq++;
          newLane({ sha: p, color: colorSeq, origin: i });
        }
      }
    } else {
      lanes[lane] = { sha: '', color: 0, origin: 0 };
    }
    trimTrailingHoles();

    snapshots[i] = lanes.map((l) => ({ ...l }));
  });

  // 连线（行 i → i+1）。每段两类线，按条目归属判断：
  //   节点出线：条目是本行 commit 的父提交 → 从节点泳道出发（首父同位时即竖线，
  //             额外父/合并占位时为拐弯曲线）；
  //   穿越线：条目在上一行快照已存在（上方有子提交的支线路过本段）→ 从自身位置出发；
  //           泳道压缩时 to 落在新位置，表现为折线。
  // 两类可并存（同一父被多条支线共享时，节点出线 + 穿越线同时存在）；
  // 首父接管原泳道时二者重合，只发一条。
  // 让位节点的出线：首父由更靠左的兄弟泳道认领时，父不在本行快照里，
  // 直接发出线拐向兄弟泳道（兄弟认领后垂直延续、随其落到共同父节点）。
  const wires: GraphWire[] = [];
  for (let i = 0; i + 1 < commits.length; i++) {
    const next = commits[i + 1];
    const parentSet = new Set(parentsOf(commits[i]));
    const target = (entry: graphLane): number => {
      if (entry.sha === next.sha) return nodes[i + 1].lane;
      const snap = snapshots[i + 1];
      for (let j = 0; j < snap.length; j++) if (snap[j].sha === entry.sha) return j;
      return -1;
    };
    const deferTo = deferred.get(i);
    if (deferTo !== undefined) {
      wires.push({ row: i, from: nodes[i].lane, to: deferTo, color: nodes[i].color, origin: nodeOrigins[i] });
    }

    snapshots[i].forEach((entry, pos) => {
      if (entry.sha === '') return; // 空洞
      const to = target(entry);
      if (to < 0) return;
      const isParent = parentSet.has(entry.sha);
      // 上方是否有来线：条目由更早的行放置（首行上方无快照，必为 false）
      const incoming = i > 0 && snapshots[i - 1].some((l) => l.sha === entry.sha);
      // 节点出线（父提交）。颜色约定：本行新开的支线（分叉）用新泳道色；
      // 已存在的支线汇入主线（合并）用子节点自己的颜色——即支线全程同色，
      // 只有真正的主线竖线用主线色
      if (isParent) {
        const outColor = incoming ? nodes[i].color : entry.color;
        wires.push({ row: i, from: nodes[i].lane, to, color: outColor, origin: nodeOrigins[i] });
      }
      // 穿越线（上方支线延续），首父接管原泳道时与节点出线重合，跳过
      if (incoming && !(isParent && pos === nodes[i].lane)) {
        wires.push({ row: i, from: pos, to, color: entry.color, origin: entry.origin });
      }
    });
  }

  return { nodes, wires };
}
