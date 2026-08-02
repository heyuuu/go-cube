// cubeApp —— Alpine.js 根组件。
// 数据：fetch /api/project/list → ApiOutput{ok,message,data:{list:[...]}} → 取 data.list
// 实时性：复用后端 gitcache，前端 30s 轮询拉快照（后端 fork 子进程异步刷新）
function cubeApp() {
  return {
    // --- 状态 ---
    projects: [],
    keyword: '',
    groupFilter: 'all',
    gitFilter: 'all',
    selected: new Set(),
    drawer: null,
    loading: false,
    error: '',

    // --- 生命周期 ---
    init() {
      this.load();
      // 30s 轮询：拉被后端 gitcache 子进程刷新的快照（对齐后端 TTL ~1min）
      setInterval(() => this.load(), 30000);
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
      } catch (e) {
        this.error = '加载失败：' + (e.message || e);
      } finally {
        this.loading = false;
      }
    },

    refresh() { return this.load(); },

    // --- 计算属性 ---
    get groups() {
      const set = new Set(this.projects.map(p => p.group));
      return [...set].sort();
    },

    get filtered() {
      const kw = this.keyword.trim().toLowerCase();
      return this.projects.filter(p => {
        if (kw && !(p.name.toLowerCase().includes(kw) || (p.path || '').toLowerCase().includes(kw))) {
          return false;
        }
        if (this.groupFilter !== 'all' && p.group !== this.groupFilter) {
          return false;
        }
        if (this.gitFilter !== 'all') {
          if (this.gitFilter !== this.gitStatusOf(p)) return false;
        }
        return true;
      });
    },

    // 把 gitInfo 归类成单一状态枚举，供筛选
    gitStatusOf(p) {
      const g = p.gitInfo;
      if (!g) return 'none';
      if (g.dirty) return 'dirty';
      if (g.ahead > 0) return 'ahead';
      if (g.behind > 0) return 'behind';
      return 'clean';
    },

    // --- 交互 ---
    toggleSelect(path) {
      if (this.selected.has(path)) this.selected.delete(path);
      else this.selected.add(path);
      // Set 的响应式触发：重新赋值（Alpine 对 Set 增删不自动追踪）
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

// 暴露到全局供 HTML x-data="cubeApp()" 引用
window.cubeApp = cubeApp;
