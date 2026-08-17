// 相对时间：与后端 cmd/helpers.go 的 prettyTime 保持同一套规则，前后端展示一致。
// 过去以「前」结尾、未来以「后」结尾；单位按秒/分钟/小时/天/月/年逐级放大，不保留零头。
export function prettyTime(iso: string | null | undefined): string {
  if (!iso) return '-';
  const t = new Date(iso).getTime();
  // 零值时间戳（Go time.Time 零值 = 0001-01-01，表示「未刷新过」）不参与相对计算
  if (Number.isNaN(t) || t <= 0) return '-';
  let secs = Math.floor((Date.now() - t) / 1000);
  let suffix = '前';
  if (secs < 0) {
    secs = -secs;
    suffix = '后';
  }

  if (secs < 60) return `${secs}秒${suffix}`;
  if (secs < 3600) return `${Math.floor(secs / 60)}分钟${suffix}`;
  if (secs < 86400) return `${Math.floor(secs / 3600)}小时${suffix}`;
  if (secs < 30 * 86400) return `${Math.floor(secs / 86400)}天${suffix}`;
  if (secs < 365 * 86400) return `${Math.floor(secs / (30 * 86400))}月${suffix}`;
  return `${Math.floor(secs / (365 * 86400))}年${suffix}`;
}

export function formatDateTime(iso: string | null | undefined): string {
  if (!iso) return '-';
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return '-';
  return d.toLocaleString('zh-CN', { hour12: false });
}
