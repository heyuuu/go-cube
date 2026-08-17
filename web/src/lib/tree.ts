// 前端目录树：把（已筛选的）扁平项目列表组织成嵌套骨架树。
// 后端 /api/project/tree 已废弃移除，tree 模式不再依赖后端——直接消费 project list，
// 且天然继承当前筛选结果（旧后端版做不到「筛完转树」）。
// 取舍：按项目 path 构建骨架，不含「无项目的空中转目录」（后端 os.ReadDir 能补全，
// 前端拿不到磁盘数据；对项目导航价值有限，可接受）。
import type { Project } from '@/api/client';

export interface TreeNode {
  name: string;
  path: string;
  kind: 'dir' | 'project';
  children: TreeNode[]; // 构建完成后同级按名称字典序排序
  project?: Project; // kind === 'project' 时挂原始项目
}

export interface TreeRow {
  node: TreeNode;
  depth: number;
  hasChildren: boolean;
  expanded: boolean; // 该行自身是否展开（根恒为 true）
  isRoot: boolean;
}

// 构建期节点：children 用 Map 便于按段查找，收尾转成排序数组
interface BuildNode {
  name: string;
  path: string;
  kind: 'dir' | 'project';
  children: Map<string, BuildNode>;
  project?: Project;
}

// twoPathCommonDir 求两条路径的最长公共目录前缀。
// 以分隔符为界逐段回退，/a/b 不会误匹配 /a/beta。
function twoPathCommonDir(a: string, b: string): string {
  // 保证 a 较短：回退只需沿 a 的分隔符推进，不必处理 a 超出 b 长度的情形
  if (a.length > b.length) [a, b] = [b, a];
  let prefix = a;
  while (prefix !== '' && prefix !== '/' && b !== prefix && !b.startsWith(prefix + '/')) {
    const idx = prefix.lastIndexOf('/', prefix.length - 2);
    prefix = idx <= 0 ? (idx === 0 ? '/' : '') : prefix.slice(0, idx);
  }
  return prefix;
}

// commonPrefixDir 求一组路径的最长公共前缀目录；单条路径取父目录；无公共前缀返回 ''。
function commonPrefixDir(paths: string[]): string {
  if (paths.length === 0) return '';
  if (paths.length === 1) {
    const idx = paths[0]!.lastIndexOf('/');
    return idx > 0 ? paths[0]!.slice(0, idx) : '/';
  }
  let prefix = paths[0]!;
  for (const p of paths.slice(1)) {
    prefix = twoPathCommonDir(prefix, p);
    if (prefix === '') return '';
  }
  return prefix;
}

// buildProjectTree 根取所有项目路径的公共前缀目录；项目路径的中间目录自动补全为 dir 节点。
// 无公共前缀（项目分散在多个根下）返回 null。
export function buildProjectTree(projects: Project[]): TreeNode | null {
  const root = commonPrefixDir(projects.map((p) => p.path));
  if (!root) return null;

  const rootBuild: BuildNode = { name: root, path: root, kind: 'dir', children: new Map() };
  for (const p of projects) {
    // 项目相对 root 的路径段；rel 为空表示项目就在 root（极少见），把根标记为项目
    const rel = p.path.slice(root.length).replace(/^\/+/, '');
    if (rel === '') {
      rootBuild.kind = 'project';
      rootBuild.project = p;
      continue;
    }
    const segs = rel.split('/');
    let cur = rootBuild;
    for (let i = 0; i < segs.length; i++) {
      const seg = segs[i]!;
      const isLast = i === segs.length - 1;
      let child = cur.children.get(seg);
      if (!child) {
        const childPath = cur.path === '/' ? `/${seg}` : `${cur.path}/${seg}`;
        child = isLast
          ? { name: seg, path: childPath, kind: 'project', children: new Map(), project: p }
          : { name: seg, path: childPath, kind: 'dir', children: new Map() };
        cur.children.set(seg, child);
      }
      cur = child;
    }
  }
  return toCompressedTreeNode(rootBuild);
}

function toTreeNode(n: BuildNode): TreeNode {
  const children = [...n.children.values()].sort((a, b) => a.name.localeCompare(b.name)).map(toTreeNode);
  return { name: n.name, path: n.path, kind: n.kind, children, ...(n.project ? { project: n.project } : {}) };
}

// toCompressedTreeNode 转成 TreeNode 并折叠单链目录（根不参与，保持 ~ 全路径展示）。
// 例：~/x 下仅 ~/x/game、~/x/game 下仅 ~/x/game/godot → 一个 x/game/godot 节点。
// 前端骨架树只含通往项目的目录，故「无其他文件」天然成立；项目节点不并入目录、分叉不折叠。
function toCompressedTreeNode(rootBuild: BuildNode): TreeNode {
  const root = toTreeNode(rootBuild);
  root.children = root.children.map(compressNode);
  return root;
}

// compressNode 自底向上折叠：目录有且仅有一个子目录时合并为「a/b/c」。
// path 取合并后最深层目录（展开态 key 与 tooltip 跟随实际目录）。
function compressNode(n: TreeNode): TreeNode {
  const node: TreeNode = { ...n, children: n.children.map(compressNode) };
  while (node.kind === 'dir' && node.children.length === 1 && node.children[0].kind === 'dir') {
    const child = node.children[0];
    node.name = `${node.name}/${child.name}`;
    node.path = child.path;
    node.children = child.children;
  }
  return node;
}

// flattenTree 拍平成可见行（depth/hasChildren/expanded），供列表渲染；根恒展开。
export function flattenTree(root: TreeNode, isExpanded: (path: string) => boolean): TreeRow[] {
  const rows: TreeRow[] = [];
  const walk = (node: TreeNode, depth: number, isRoot: boolean) => {
    const hasChildren = node.children.length > 0;
    const expanded = isRoot || isExpanded(node.path);
    rows.push({ node, depth, hasChildren, expanded, isRoot });
    if (hasChildren && expanded) {
      for (const c of node.children) walk(c, depth + 1, false);
    }
  };
  walk(root, 0, true);
  return rows;
}

// collectExpandablePaths 收集所有含子节点的目录 path（全展开用）
export function collectExpandablePaths(root: TreeNode): string[] {
  const paths: string[] = [];
  const walk = (n: TreeNode) => {
    if (n.children.length > 0) {
      paths.push(n.path);
      for (const c of n.children) walk(c);
    }
  };
  walk(root);
  return paths;
}
