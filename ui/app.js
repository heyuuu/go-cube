// cubeApp —— Alpine.js 根组件入口。
//
// 拆分约定：各视图的状态/方法分散在 views/*.js，通过 cubeAppExtras 注册扩展，
// 本文件定义主干（公共状态 + init + 扩展合并）。加载顺序：
//   app.js（本文件，定义 cubeApp + extras 机制）→ views/*.js（注册扩展）→ alpine.min.js
//
// 数据：fetch /api/project/list → ApiOutput{ok,message,data:{list, gitCacheUpdated}}
// 实时性：复用后端 gitcache，前端 30s 轮询拉快照（后端 fork 子进程异步刷新）

// 扩展注册表：各视图文件 push 自己的状态/方法/getter
// 挂 window 确保跨 defer 脚本文件可访问（const 顶层在非 module 脚本是全局的，但显式挂 window 更稳）
window.cubeAppExtras = window.cubeAppExtras || [];
function registerCubeAppExtras(extra) { window.cubeAppExtras.push(extra); }

function cubeApp() {
  // 公共状态（所有视图共享）
  const base = {
    // 视图切换
    view: 'projects',  // 当前视图：projects / tree / config

    // 公共数据
    projects: [],
    openers: [],
    gitCacheUpdated: null,
    loading: false,
    error: '',

    // 详情抽屉（projects/tree 共用）
    drawer: null,
    drawerOpenMenu: false,
    openMenuPath: null,
    opening: {},

    // --- 生命周期 ---
    init() {
      this.load();
      this.loadOpeners();
      setInterval(() => this.load(), 30000);
    },

    // 视图切换：切到 tree/config 时按需加载
    switchView(v) {
      this.view = v;
      if (v === 'tree' && !this.treeRoot && !this.treeLoading) this.loadTree();
      if (v === 'config' && !this.config && !this.configLoading) this.loadConfig();
    },

    // --- 公共数据加载 ---
    async load() {
      this.loading = true;
      this.error = '';
      try {
        const res = await fetch('/api/project/list');
        const json = await res.json();
        if (!json.ok) throw new Error(json.message || '请求失败');
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
        if (json.ok) this.openers = json.data?.list || [];
      } catch (e) {}
    },

    refresh() { return this.load(); },

    // --- 公共交互 ---
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

    // 缓存更新时间相对描述
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
  };

  // 合并各视图扩展（保留 getter/setter 描述符，Object.assign 会丢失它们）
  for (const extra of window.cubeAppExtras || []) {
    const descs = Object.getOwnPropertyDescriptors(extra);
    Object.defineProperties(base, descs);
  }
  return base;
}
window.cubeApp = cubeApp;
