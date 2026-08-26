// settings 页 Config 分区：config.json 全量只读展示（过渡形态，提案 1025）。
// 各域配置管理能力到位后（openers 已走 Opener 分区），对应区块逐步从这里收编为可编辑分区。
import { ErrorBanner } from '@/components/error-banner';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { useConfig } from '@/queries/config';

// 配置小节：标题 + 表格（空数据显示占位行）
function ConfigTable({ title, sub, head, rows }: { title: string; sub?: string; head: string[]; rows: string[][] }) {
  return (
    <section>
      <div className="mb-2 flex items-baseline gap-2">
        <h2 className="text-sm font-medium">{title}</h2>
        {sub && <span className="text-xs text-muted-foreground">{sub}</span>}
      </div>
      <div className="rounded-lg border">
        <Table>
          <TableHeader>
            <TableRow>
              {head.map((h) => (
                <TableHead key={h}>{h}</TableHead>
              ))}
            </TableRow>
          </TableHeader>
          <TableBody>
            {rows.length === 0 ? (
              <TableRow>
                <TableCell colSpan={head.length} className="text-xs text-muted-foreground">
                  暂无配置
                </TableCell>
              </TableRow>
            ) : (
              rows.map((row, i) => (
                <TableRow key={i}>
                  {row.map((cell, j) => (
                    <TableCell key={j} className={j === 0 ? 'font-medium' : 'font-mono text-xs'}>
                      {cell === '' ? '-' : cell}
                    </TableCell>
                  ))}
                </TableRow>
              ))
            )}
          </TableBody>
        </Table>
      </div>
    </section>
  );
}

export function ConfigSection() {
  const config = useConfig();
  const cfg = config.data;

  return (
    <div className="flex flex-col gap-6">
      <p className="text-xs text-muted-foreground">
        config.json 只读事实（启动期读一次）；修改请编辑 config.json 或使用 CLI
      </p>
      {config.error && <ErrorBanner message={`加载失败：${config.error.message}`} />}
      {config.isPending && <div className="text-xs text-muted-foreground">加载中…</div>}

      {cfg && (
        <>
          <section>
            <h2 className="mb-2 text-sm font-medium">基本信息</h2>
            <dl className="grid grid-cols-[auto_1fr] gap-x-6 gap-y-1.5 rounded-lg border p-4 text-xs">
              <dt className="text-muted-foreground">dataDir</dt>
              <dd className="font-mono">{cfg.dataDir || '-'}</dd>
              <dt className="text-muted-foreground">log.path</dt>
              <dd className="font-mono">{cfg.log?.path || '-'}</dd>
              <dt className="text-muted-foreground">log.level</dt>
              <dd className="font-mono">{cfg.log?.level || '-'}</dd>
              <dt className="text-muted-foreground">log.format</dt>
              <dd className="font-mono">{cfg.log?.format || '-'}</dd>
            </dl>
          </section>

          <ConfigTable
            title="扫描规则（project.scan）"
            sub="扫描根目录与深度"
            head={['group', 'path', 'maxDepth']}
            rows={(cfg.project?.scan ?? []).map((r) => [r.group, r.path, String(r.maxDepth)])}
          />

          <ConfigTable
            title="clone 路由（project.clone）"
            sub="按 repoHost / repoPrefix 匹配落地路径"
            head={['repoHost', 'repoPrefix', 'localPath']}
            rows={(cfg.project?.clone ?? []).map((r) => [r.repoHost, r.repoPrefix, r.localPath])}
          />
        </>
      )}
    </div>
  );
}
