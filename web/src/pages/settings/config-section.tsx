// settings 页 Config 分区：config.json 只读展示（过渡形态，提案 1025）。
// 各域配置管理能力到位后（openers → Opener 分区、scan/clone 规则 → 项目·扫描 分区），
// 对应区块已从这里收编为可编辑分区。
import { ErrorBanner } from '@/components/error-banner';
import { useConfig } from '@/queries/config';

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
      )}
    </div>
  );
}
