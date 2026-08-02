// projects 视图扩展：筛选、git 状态、行内打开
registerCubeAppExtras({
  // 筛选状态
  keyword: '',
  groupFilter: [],   // group 多选：空数组 = 全部
  gitFilter: 'all',  // git 单选：'all' = 全部
  tagFilter: 'all',  // tag 单选：'all' = 全部
  selected: new Set(),

  gitStatuses: [
    { value: 'clean', label: 'clean' },
    { value: 'dirty', label: 'dirty' },
    { value: 'ahead', label: 'ahead' },
    { value: 'behind', label: 'behind' },
    { value: 'none', label: '未采集' },
  ],

  // --- getters ---
  get groups() {
    const set = new Set(this.projects.map(p => p.group));
    return [...set].sort();
  },

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
      if (this.groupFilter.length > 0 && !this.groupFilter.includes(p.group)) return false;
      if (this.gitFilter !== 'all' && this.gitFilter !== this.gitStatusOf(p)) return false;
      if (this.tagFilter !== 'all' && !(p.tags || []).includes(this.tagFilter)) return false;
      return true;
    });
  },

  // --- 方法 ---
  gitStatusOf(p) {
    const g = p.gitInfo;
    if (!g) return 'none';
    if (g.dirty) return 'dirty';
    if (g.ahead > 0) return 'ahead';
    if (g.behind > 0) return 'behind';
    return 'clean';
  },

  toggleChip(key, value) {
    const arr = this[key];
    const idx = arr.indexOf(value);
    if (idx >= 0) arr.splice(idx, 1);
    else arr.push(value);
    this[key] = [...arr];
  },

  toggleSelect(path) {
    if (this.selected.has(path)) this.selected.delete(path);
    else this.selected.add(path);
    this.selected = new Set(this.selected);
  },

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
});
