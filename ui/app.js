// cubeApp —— Alpine.js 根组件入口。
//
// 拆分约定：各视图的状态/方法分散在 views/*.js，通过 cubeAppExtras 注册扩展，
// 本文件定义主干（公共状态 + init + 扩展合并）。加载顺序：
//   app.js（本文件，定义 cubeApp + extras 机制）→ views/*.js（注册扩展）→ alpine.min.js
//
// 数据：fetch /api/project/list → ApiOutput{ok,message,data:{list, scanUpdatedAt, gitUpdatedAt}}
// 实时性：复用后端 gitcache，前端 30s 轮询拉快照（后端 fork 子进程异步刷新）

// 扩展注册表：各视图文件 push 自己的状态/方法/getter
// 挂 window 确保跨 defer 脚本文件可访问（const 顶层在非 module 脚本是全局的，但显式挂 window 更稳）
window.cubeAppExtras = window.cubeAppExtras || [];
function registerCubeAppExtras(extra) { window.cubeAppExtras.push(extra); }

function cubeApp() {
  // 公共状态（所有视图共享）
  const base = {
    // 视图切换
    view: 'projects',  // 当前视图：projects / config
    displayMode: 'table',  // projects 视图内的显示模式：table / tree

    // 公共数据
    projects: [],
    openers: [],
    scanUpdatedAt: null,
    gitUpdatedAt: null,
    loading: false,
    error: '',

    // 详情抽屉（projects/tree 共用）
    drawer: null,
    drawerOpenMenu: false,
    openMenuPath: null,
    opening: {},

    // --- 生命周期 ---
    init() {
      // 从 URL hash 恢复视图状态（支持 F5 刷新停在当前页、分享链接直达）
      this.applyHash();
      window.addEventListener('hashchange', () => this.applyHash());

      this.load();
      this.loadOpeners();
      setInterval(() => this.load(), 30000);
    },

    // --- hash 路由 ---
    // 设计：#/projects[/{table|tree}] | #/config （视图）；#/p/<encoded-path> （详情抽屉）
    // hash 模式而非 history：cube server 无 SPA fallback 路由，hash 刷新只请求 / 拿 index.html，零后端改动。
    //
    // 状态 → URL（switchView/switchDisplay/openDrawer/closeDrawer 调用）
    updateHash() {
      let hash;
      if (this.drawer) {
        hash = '#/p/' + encodeURIComponent(this.drawer.path);
      } else if (this.view === 'projects') {
        hash = '#/projects' + (this.displayMode === 'tree' ? '/tree' : '');
      } else {
        hash = '#/' + (this.view || 'projects');
      }
      if (location.hash !== hash) {
        location.hash = hash;
      }
    },
    // URL → 状态（init 恢复 + hashchange 监听前进/后退）
    applyHash() {
      const route = this.parseHash();
      // 视图切换
      if (route.view && route.view !== this.view) {
        this.switchView(route.view, { skipHash: true });
      }
      // projects 视图内的显示模式
      if (route.view === 'projects' && route.mode && route.mode !== this.displayMode) {
        this.switchDisplay(route.mode, { skipHash: true });
      }
      // 抽屉
      if (route.drawerPath) {
        const p = this.projects.find(x => x.path === route.drawerPath);
        if (p) {
          this.drawer = p;
          return;
        }
      }
      // 无抽屉路由则关闭（处理浏览器后退关抽屉）
      if (!route.drawerPath && this.drawer) {
        this.drawer = null;
      }
    },
    parseHash() {
      const h = location.hash.replace(/^#\/?/, ''); // 去掉 # 和开头 /
      if (!h) return { view: 'projects' };
      const parts = h.split('/');
      if (parts[0] === 'p' && parts[1]) {
        return { drawerPath: decodeURIComponent(parts[1]) };
      }
      if (parts[0] === 'projects') {
        return { view: 'projects', mode: parts[1] === 'tree' ? 'tree' : 'table' };
      }
      if (parts[0] === 'config') {
        return { view: 'config' };
      }
      return { view: 'projects' };
    },

    // 视图切换：切到 config 时按需加载
    switchView(v, opts = {}) {
      this.view = v;
      if (v === 'config' && !this.config && !this.configLoading) this.loadConfig();
      if (!opts.skipHash) {
        // 切视图时关抽屉（避免抽屉跨视图残留）
        this.drawer = null;
        this.updateHash();
      }
    },

    // projects 视图内的显示模式切换（table / tree）
    switchDisplay(mode, opts = {}) {
      this.displayMode = mode;
      // 切到 tree 时重置展开状态（树基于 filtered 重建）
      if (mode === 'tree') {
        this.treeExpanded = {};
      }
      if (!opts.skipHash) {
        this.updateHash();
      }
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
        this.scanUpdatedAt = json.data?.scanUpdatedAt || null;
        this.gitUpdatedAt = json.data?.gitUpdatedAt || null;
        // 首次加载完成后，若 URL 指向某个 project 详情，补开抽屉（init 时 projects 还空）
        if (!this._drawerResolved) {
          this._drawerResolved = true;
          this.applyHash();
        }
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
    openDrawer(p) {
      this.drawer = p;
      this.updateHash();
    },
    closeDrawer() {
      this.drawer = null;
      this.updateHash();
    },

    formatTime(s) {
      if (!s) return '-';
      const d = new Date(s);
      if (isNaN(d)) return s;
      return d.toLocaleString('zh-CN', { hour12: false });
    },

    async copyPath(path) {
      try { await navigator.clipboard.writeText(path); } catch (e) {}
    },

    // prettyPath 把绝对路径里的 HOME 前缀替换为 ~，缩短显示（对应后端 pathkit.PrettyPath）。
    // 浏览器无法直接读 HOME，用启发式：取所有已知项目路径的最长公共「/Users/<user>」前缀作为 HOME 近似。
    // 推断失败（路径形态非标准）则原样返回。
    prettyPath(p) {
      if (!p) return '';
      const home = this.guessHome();
      if (home && p === home) return '~';
      if (home && p.startsWith(home + '/')) return '~' + p.slice(home.length);
      return p;
    },
    // guessHome 从已加载项目的路径推断 HOME（缓存）。MAC/Linux 路径形如 /Users/<user>/... 或 /home/<user>/...。
    guessHome() {
      if (this._homeCache !== undefined) return this._homeCache;
      let home = '';
      for (const p of (this.projects || []).map(x => x.path)) {
        const m = p.match(/^(\/(?:Users|home)\/[^/]+)\//);
        if (m) { home = m[1]; break; }
      }
      this._homeCache = home; // 空串也缓存（避免重复尝试）
      return home;
    },

    // 项目列表刷新时间相对描述
    get scanUpdatedAtText() {
      if (!this.scanUpdatedAt) return '-';
      const diff = Date.now() - new Date(this.scanUpdatedAt).getTime();
      if (isNaN(diff)) return '-';
      const min = Math.floor(diff / 60000);
      if (min < 1) return '刚刚';
      if (min < 60) return min + ' 分钟前';
      const hr = Math.floor(min / 60);
      if (hr < 24) return hr + ' 小时前';
      return Math.floor(hr / 24) + ' 天前';
    },

    // git 缓存刷新时间相对描述
    get gitUpdatedAtText() {
      if (!this.gitUpdatedAt) return '-';
      const diff = Date.now() - new Date(this.gitUpdatedAt).getTime();
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
