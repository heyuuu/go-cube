// tree 视图扩展：目录树加载、展开/折叠、扁平化
registerCubeAppExtras({
  treeRoot: null,
  treeExpanded: {},
  treeLoading: false,
  treeError: '',

  async loadTree() {
    this.treeLoading = true;
    this.treeError = '';
    try {
      const res = await fetch('/api/project/tree');
      const json = await res.json();
      if (!json.ok) throw new Error(json.message || '请求失败');
      this.treeRoot = json.data;
      this.treeExpanded = { [this.treeRoot.path]: true };
    } catch (e) {
      this.treeError = '加载目录树失败：' + (e.message || e);
    } finally {
      this.treeLoading = false;
    }
  },

  get flatTree() {
    if (!this.treeRoot) return [];
    const rows = [];
    const walk = (node, depth, isRoot) => {
      const hasChildren = node.children && node.children.length > 0;
      const expanded = isRoot ? true : !!this.treeExpanded[node.path];
      rows.push({
        name: node.name,
        path: node.path,
        kind: isRoot ? 'dir' : node.kind,
        depth,
        hasChildren,
        expanded,
        isRoot,
      });
      if (node.children && (isRoot || this.treeExpanded[node.path])) {
        for (const c of node.children) walk(c, depth + 1, false);
      }
    };
    walk(this.treeRoot, 0, true);
    return rows;
  },

  toggleTreeNode(row) {
    if (!row.hasChildren) return;
    this.treeExpanded[row.path] = !this.treeExpanded[row.path];
    this.treeExpanded = { ...this.treeExpanded };
  },

  expandAllTree() {
    const all = {};
    const walk = (n) => {
      if (n.children && n.children.length > 0) {
        all[n.path] = true;
        n.children.forEach(walk);
      }
    };
    walk(this.treeRoot);
    this.treeExpanded = all;
  },

  collapseAllTree() {
    this.treeExpanded = { [this.treeRoot?.path]: true };
  },
});
