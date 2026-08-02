// cubeApp —— Alpine.js 根组件。
// 数据：fetch /api/project/list → ApiOutput{ok,message,data:{list, gitCacheUpdated}}
// 实时性：复用后端 gitcache，前端 30s 轮询拉快照（后端 fork 子进程异步刷新）
function cubeApp() {
  return {
    // --- 状态 ---
    view: 'projects',  // 当前视图：projects / tree
    projects: [],
    keyword: '',
    groupFilter: [],   // group 多选：空数组 = 全部
    gitFilter: 'all',  // git 单选：'all' = 全部，否则为具体状态（clean/dirty/ahead/behind/none）
    tagFilter: 'all',  // tag 单选：'all' = 全部，否则为具体 tag（git/git-worktree/godot...）
    gitStatuses: [
      { value: 'clean', label: 'clean' },
      { value: 'dirty', label: 'dirty' },
      { value: 'ahead', label: 'ahead' },
      { value: 'behind', label: 'behind' },
      { value: 'none', label: '未采集' },
    ],
    selected: new Set(),
    drawer: null,
    loading: false,
    error: '',

    // opener 列表 + 缓存更新时间（来自后端）
    openers: [],
    gitCacheUpdated: null,
    openMenuPath: null,   // 当前展开「打开」下拉的行 path（同时只展开一个）
    drawerOpenMenu: false, // 抽屉内的「打开方式」下拉
    opening: {},          // 正在打开中的 path → bool，禁用按钮防重复

    // Tree 视图状态
    treeRoot: null,       // 后端返回的原始树
    treeExpanded: {},     // path → bool，节点展开状态
    treeLoading: false,
    treeError: '',

    // --- 生命周期 ---
    init() {
      this.load();
      this.loadOpeners();
      // 30s 轮询：拉被后端 gitcache 子进程刷新的快照（对齐后端 TTL ~1min）
      setInterval(() => this.load(), 30000);
    },

    // 视图切换：切到 tree 时按需加载（目录树变化不频繁，无需轮询）
    switchView(v) {
      this.view = v;
      if (v === 'tree' && !this.treeRoot && !this.treeLoading) {
        this.loadTree();
      }
    },

    // --- Tree 视图 ---
    async loadTree() {
      this.treeLoading = true;
      this.treeError = '';
      try {
        const res = await fetch('/api/project/tree');
        const json = await res.json();
        if (!json.ok) throw new Error(json.message || '请求失败');
        this.treeRoot = json.data;
        // 默认展开根节点 + 第一层
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
        if (!isRoot || hasChildren || true) {  // 根节点也显示
          rows.push({
            name: node.name,
            path: node.path,
            kind: isRoot ? 'dir' : node.kind,  // 根节点当 dir 样式
            depth,
            hasChildren,
            expanded,
            isRoot,
          });
        }
        // 根节点总是展开；其余仅展开时下钻
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
      // 触发 Alpine 响应式（对象属性新增）
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
      this.treeExpanded = { [this.treeRoot?.path]: true };  // 仅保留根展开
    },

    // --- 数据加载 ---
    async load() {
      this.loading = true;
      this.error = '';
      try {
        const res = await fetch('/api/project/list');
        const json = await res.json();
        if (!json.ok) {
          throw new Error(json.message || '请求失败');
        }
        this.projects = json.data?.list || [];
        this.gitCacheUpdated = json.data?.gitCacheUpdated || null;
      } catch (e) {
        this.error = '加载失败：' + (e.message || e);
      } finally {
        this.loading = false;
      }
    },

    async loadOpeners() {
      try {
        const res = await fetch('/api/opener/list');
        const json = await res.json();
        if (json.ok) {
          this.openers = json.data?.list || [];
        }
      } catch (e) {}
    },

    refresh() { return this.load(); },

    // --- 计算属性 ---
    get groups() {
      const set = new Set(this.projects.map(p => p.group));
      return [...set].sort();
    },

    // 聚合所有项目出现过的 tag（git / git-worktree / godot ...）
    get tags() {
      const set = new Set();
      this.projects.forEach(p => (p.tags || []).forEach(t => set.add(t)));
      return [...set].sort();
    },

    get filtered() {
      const kw = this.keyword.trim().toLowerCase();
      return this.projects.filter(p => {
        if (kw && !(p.name.toLowerCase().includes(kw) || (p.path || '').toLowerCase().includes(kw))) {
          return false;
        }
        if (this.groupFilter.length > 0 && !this.groupFilter.includes(p.group)) {
          return false;
        }
        // git 单选：'all' = 全部，否则需精确匹配
        if (this.gitFilter !== 'all' && this.gitFilter !== this.gitStatusOf(p)) {
          return false;
        }
        // tag 单选：'all' = 全部，否则需项目 tags 包含该 tag
        if (this.tagFilter !== 'all' && !(p.tags || []).includes(this.tagFilter)) {
          return false;
        }
        return true;
      });
    },

    // 缓存更新时间的相对描述（如「3 分钟前」），无缓存返回 '-'
    get gitCacheUpdatedText() {
      if (!this.gitCacheUpdated) return '-';
      const diff = Date.now() - new Date(this.gitCacheUpdated).getTime();
      if (isNaN(diff)) return '-';
      const min = Math.floor(diff / 60000);
      if (min < 1) return '刚刚';
      if (min < 60) return min + ' 分钟前';
      const hr = Math.floor(min / 60);
      if (hr < 24) return hr + ' 小时前';
      return Math.floor(hr / 24) + ' 天前';
    },

    // toggleChip：点击单个 chip 切换选中状态（点击「全部」则直接清空数组）
    toggleChip(key, value) {
      const arr = this[key];
      const idx = arr.indexOf(value);
      if (idx >= 0) {
        arr.splice(idx, 1);  // 已选 → 取消
      } else {
        arr.push(value);     // 未选 → 加入
      }
      this[key] = [...arr];  // 触发 Alpine 响应式
    },

    gitStatusOf(p) {
      const g = p.gitInfo;
      if (!g) return 'none';
      if (g.dirty) return 'dirty';
      if (g.ahead > 0) return 'ahead';
      if (g.behind > 0) return 'behind';
      return 'clean';
    },

    // --- 打开项目 ---
    toggleOpenMenu(path) {
      this.openMenuPath = this.openMenuPath === path ? null : path;
    },

    async openProject(path, app) {
      if (this.opening[path + app]) return;
      this.opening[path + app] = true;
      this.openMenuPath = null;
      try {
        const res = await fetch('/api/project/open', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ path, app }),
        });
        const json = await res.json();
        if (!json.ok) throw new Error(json.message || '打开失败');
      } catch (e) {
        this.error = '打开失败：' + (e.message || e);
      } finally {
        this.opening[path + app] = false;
      }
    },

    // --- 交互 ---
    toggleSelect(path) {
      if (this.selected.has(path)) this.selected.delete(path);
      else this.selected.add(path);
      this.selected = new Set(this.selected);
    },

    openDrawer(p) { this.drawer = p; },

    formatTime(s) {
      if (!s) return '-';
      const d = new Date(s);
      if (isNaN(d)) return s;
      return d.toLocaleString('zh-CN', { hour12: false });
    },

    async copyPath(path) {
      try { await navigator.clipboard.writeText(path); } catch (e) {}
    },
  };
}

window.cubeApp = cubeApp;
