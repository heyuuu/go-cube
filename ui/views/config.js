// config 视图扩展：配置查看 + scan/clone/opener 规则增删
registerCubeAppExtras({
  config: null,
  configLoading: false,
  configError: '',
  configTab: 'scan',  // scan / clone / opener

  // 新增表单（各类型一组输入）
  newScan: { group: '', path: '', maxDepth: 2 },
  newClone: { repoHost: '', repoPrefix: '', localPath: '' },
  newOpener: { name: '', cmd: '', roles: 'open-dir' },

  async loadConfig() {
    this.configLoading = true;
    this.configError = '';
    try {
      const res = await fetch('/api/config');
      const json = await res.json();
      if (!json.ok) throw new Error(json.message || '请求失败');
      this.config = json.data;
    } catch (e) {
      this.configError = '加载配置失败：' + (e.message || e);
    } finally {
      this.configLoading = false;
    }
  },

  get scanRules() { return this.config?.project?.scan || []; },
  get cloneRules() { return this.config?.project?.clone || []; },
  get openerRules() { return this.config?.openers || []; },

  // --- scan 增删 ---
  async addScan() {
    const r = { ...this.newScan };
    if (!r.group || !r.path) { this.configError = 'group 和 path 不能为空'; return; }
    const data = await this.configMutate('/api/config/scan', 'POST', r);
    if (data) {
      this.config.project.scan = data.scan;
      this.newScan = { group: '', path: '', maxDepth: 2 };
    }
  },

  async delScan(rule) {
    const data = await this.configMutate('/api/config/scan', 'DELETE', { group: rule.group, path: rule.path });
    if (data) this.config.project.scan = data.scan;
  },

  // --- clone 增删 ---
  async addClone() {
    const r = { ...this.newClone };
    if (!r.repoHost || !r.localPath) { this.configError = 'repoHost 和 localPath 不能为空'; return; }
    const data = await this.configMutate('/api/config/clone', 'POST', r);
    if (data) {
      this.config.project.clone = data.clone;
      this.newClone = { repoHost: '', repoPrefix: '', localPath: '' };
    }
  },

  async delClone(rule) {
    const data = await this.configMutate('/api/config/clone', 'DELETE', rule);
    if (data) this.config.project.clone = data.clone;
  },

  // --- opener 增删 ---
  async addOpener() {
    const cmdStr = this.newOpener.cmd.trim();
    if (!this.newOpener.name || !cmdStr) { this.configError = 'name 和 cmd 不能为空'; return; }
    // cmd 用空格分词（简单处理，复杂命令请直接编辑 config.json）
    const r = {
      name: this.newOpener.name,
      cmd: cmdStr.split(/\s+/),
      roles: this.newOpener.roles.split(/[,\s]+/).filter(x => x),
    };
    const data = await this.configMutate('/api/config/opener', 'POST', r);
    if (data) {
      this.config.openers = data;
      this.newOpener = { name: '', cmd: '', roles: 'open-dir' };
    }
  },

  async delOpener(name) {
    const data = await this.configMutate('/api/config/opener', 'DELETE', { name });
    if (data) this.config.openers = data;
  },

  // --- 公共：配置增删请求 ---
  async configMutate(url, method, body) {
    this.configError = '';
    try {
      const res = await fetch(url, {
        method,
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body),
      });
      const json = await res.json();
      if (!json.ok) throw new Error(json.message || '操作失败');
      return json.data;
    } catch (e) {
      this.configError = (method === 'POST' ? '添加失败：' : '删除失败：') + (e.message || e);
      return null;
    }
  },
});
