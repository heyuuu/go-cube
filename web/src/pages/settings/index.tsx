import { useSearchParams } from 'react-router';

import { PageHeader } from '@/components/page-header';
import { cn } from '@/lib/utils';

import { ConfigSection } from './config-section';
import { OpenerSection } from './opener-section';

// settings 分区清单（提案 1025）：Config（config.json 只读展示的过渡分区）为默认分区，
// Opener 可编辑；各域配置管理能力到位后逐步把 Config 内容收编为可编辑分区（如「项目·扫描」）。
// key 同时是 ?section= 的取值与内容区分发键。
const SECTIONS = [
  { key: 'config', label: 'Config' },
  { key: 'opener', label: 'Opener' },
  { key: 'scan', label: '项目·扫描', placeholder: '扫描规则编辑随 project 域提案到货（当前在 Config 分区只读查看）' },
] as const;

const DEFAULT_SECTION = SECTIONS[0].key;
type SectionKey = (typeof SECTIONS)[number]['key'];

export function SettingsPage() {
  const [params, setParams] = useSearchParams();
  const section = (params.get('section') ?? DEFAULT_SECTION) as SectionKey;
  const current = SECTIONS.find((s) => s.key === section) ?? SECTIONS[0];

  return (
    <div>
      <PageHeader
        title="设置"
        meta="Config 分区为 config.json 只读事实；Opener 等用户可管理数据（settings.json）保存即生效"
      />
      <div className="flex gap-6 px-6 pb-6">
        {/* 页内左侧分区导航：分区写 URL（?section=），可刷新/直达；默认分区不写参数 */}
        <nav className="flex w-36 shrink-0 flex-col gap-0.5">
          {SECTIONS.map((s) => (
            <button
              key={s.key}
              type="button"
              onClick={() => setParams(s.key === DEFAULT_SECTION ? {} : { section: s.key }, { replace: true })}
              className={cn(
                'rounded-md px-2.5 py-1.5 text-left text-sm text-muted-foreground transition-colors hover:bg-sidebar-accent hover:text-sidebar-accent-foreground',
                s.key === current.key && 'bg-sidebar-accent font-medium text-sidebar-accent-foreground',
              )}
            >
              {s.label}
            </button>
          ))}
        </nav>
        <div className="min-w-0 flex-1">
          {current.key === 'config' && <ConfigSection />}
          {current.key === 'opener' && <OpenerSection />}
          {'placeholder' in current && (
            <div className="rounded-lg border p-8 text-center text-xs text-muted-foreground">{current.placeholder}</div>
          )}
        </div>
      </div>
    </div>
  );
}
