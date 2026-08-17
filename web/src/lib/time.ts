// 相对时间：列表页「xx 分钟前」类的新鲜度展示
export function formatRelative(iso: string | null | undefined): string {
  if (!iso) return '-';
  const t = new Date(iso).getTime();
  // 零值时间戳（Go time.Time 零值 = 0001-01-01，表示「未刷新过」）不参与相对计算
  if (Number.isNaN(t) || t <= 0) return '-';
  const min = Math.floor((Date.now() - t) / 60000);
  if (min < 1) return '刚刚';
  if (min < 60) return `${min} 分钟前`;
  const hr = Math.floor(min / 60);
  if (hr < 24) return `${hr} 小时前`;
  return `${Math.floor(hr / 24)} 天前`;
}

export function formatDateTime(iso: string | null | undefined): string {
  if (!iso) return '-';
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return '-';
  return d.toLocaleString('zh-CN', { hour12: false });
}
