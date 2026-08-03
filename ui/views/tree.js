// tree 显示模式扩展：基于 filtered（已筛选项目）构建前端目录树。
//
// 与旧版差异：旧版独立 fetch /api/project/tree，不受筛选影响、也无法"筛完转 tree"。
// 合并到 projects 视图后，tree 直接消费 filtered，筛选即时生效。
// 取舍：前端按项目 path 构建骨架树，不含"无项目的空中转目录"（后端 BuildTree 会
// os.ReadDir 补全这种目录，前端做不到；但对项目导航价值有限，可接受）。
registerCubeAppExtras({
  treeExpanded: {},  // path -> bool，记录每个目录节点的展开状态

  // buildFilteredTree 把 filtered（扁平项目数组）组织成嵌套树。
  // 根节点取所有项目路径的最长公共前缀目录；项目路径的中间目录自动补全为 dir 节点。
  get filteredTreeRoot() {
    const projects = this.filtered;
    if (!projects || projects.length === 0) return null;

    // 计算公共前缀作为树根
    const paths = projects.map(p => p.path);
    const root = this.commonPrefixDir(paths);
    if (!root) return null;

    // 按项目路径构建骨架：每个项目路径拆成段，挂到 root 下
    // 用 map 存子节点，key=目录名，value=节点
    const buildNode = (name, path) => ({ name, path, kind: 'dir', children: {} });
    const rootNode = { name: root, path: root, kind: 'dir', children: {} };

    for (const p of projects) {
      // 项目相对 root 的路径段
      let rel = p.path.slice(root.length).replace(/^\/+/, '');
      // rel 为空表示项目就在 root（极少见），直接挂 root
      if (rel === '') {
        rootNode.kind = 'project';
        rootNode.project = p;
        continue;
      }
      const segs = rel.split('/').filter(Boolean);
      let cur = rootNode;
      for (let i = 0; i < segs.length; i++) {
        const seg = segs[i];
        const isLast = i === segs.length - 1;
        if (!cur.children[seg]) {
          const childPath = cur.path === '/' ? '/' + seg : cur.path + '/' + seg;
          cur.children[seg] = isLast
            ? { name: seg, path: childPath, kind: 'project', project: p }
            : { name: seg, path: childPath, kind: 'dir', children: {} };
        }
        cur = cur.children[seg];
      }
    }
    return rootNode;
  },

  // flatFilteredTree 把嵌套树拍平成行（带 depth/hasChildren/expanded），供 x-for 渲染。
  get flatFilteredTree() {
    const root = this.filteredTreeRoot;
    if (!root) return [];
    const rows = [];
    const walk = (node, depth, isRoot) => {
      // children 从 map 转 sorted array
      const childNodes = Object.values(node.children || {}).sort((a, b) =>
        a.name.localeCompare(b.name)
      );
      const hasChildren = childNodes.length > 0;
      const expanded = isRoot ? true : !!this.treeExpanded[node.path];
      rows.push({
        name: node.name,
        path: node.path,
        kind: node.kind,
        depth,
        hasChildren,
        expanded,
        isRoot,
        project: node.project || null,
      });
      if (childNodes.length > 0 && (isRoot || this.treeExpanded[node.path])) {
        for (const c of childNodes) walk(c, depth + 1, false);
      }
    };
    walk(root, 0, true);
    return rows;
  },

  // commonPrefixDir 求一组路径的最长公共前缀目录（前端版，对应后端 pathkit.CommonPrefix）。
  // 缩短按 / 切分进行，结果天然是目录边界，无需二次截断。无公共前缀返回空串。
  commonPrefixDir(paths) {
    if (paths.length === 0) return '';
    let prefix = paths[0];
    // 单条路径：取其父目录作为根（否则根就是项目本身，树没意义）
    if (paths.length === 1) {
      const idx = prefix.lastIndexOf('/');
      return idx > 0 ? prefix.slice(0, idx) : '/';
    }
    for (let i = 1; i < paths.length; i++) {
      while (paths[i].indexOf(prefix) !== 0) {
        const idx = prefix.lastIndexOf('/');
        if (idx <= 0) return '';
        prefix = prefix.slice(0, idx);
      }
    }
    return prefix;
  },

  toggleTreeNode(row) {
    if (!row.hasChildren) return;
    this.treeExpanded[row.path] = !this.treeExpanded[row.path];
    this.treeExpanded = { ...this.treeExpanded };
  },

  expandAllTree() {
    const all = {};
    const walk = (node) => {
      const children = Object.values(node.children || {});
      if (children.length > 0) {
        all[node.path] = true;
        children.forEach(walk);
      }
    };
    const root = this.filteredTreeRoot;
    if (root) walk(root);
    this.treeExpanded = all;
  },

  collapseAllTree() {
    this.treeExpanded = {};
  },
});
