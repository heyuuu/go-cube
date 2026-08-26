// lucide 图标选择器：搜索过滤 + 网格点选（Opener 表单 icon 编辑用）。
// icons 映射键是 PascalCase，存储值是 kebab-case，此处预计算双名做过滤与高亮。
import { icons, type LucideIcon } from 'lucide-react';
import { useMemo, useState } from 'react';

import { Input } from '@/components/ui/input';
import { cn } from '@/lib/utils';

function pascalToKebab(name: string): string {
  return name.replace(/([a-z0-9])([A-Z])/g, '$1-$2').toLowerCase();
}

type Entry = { kebab: string; Ico: LucideIcon };
const ALL: Entry[] = Object.entries(icons).map(([pascal, Ico]) => ({ kebab: pascalToKebab(pascal), Ico }));
const MAX_SHOWN = 96;

export function LucideIconPicker({ value, onChange }: { value: string; onChange: (kebab: string) => void }) {
  const [query, setQuery] = useState('');
  const hits = useMemo(() => {
    const q = query.trim().toLowerCase();
    const list = q ? ALL.filter((e) => e.kebab.includes(q)) : ALL;
    return list.slice(0, MAX_SHOWN);
  }, [query]);

  return (
    <div className="flex flex-col gap-1.5">
      <Input
        value={query}
        onChange={(e) => setQuery(e.target.value)}
        placeholder="搜索图标（如 folder / git / terminal）"
      />
      <div className="grid max-h-52 grid-cols-8 gap-1 overflow-y-auto rounded-md border p-2">
        {hits.map(({ kebab, Ico }) => (
          <button
            key={kebab}
            type="button"
            title={kebab}
            aria-label={kebab}
            aria-pressed={kebab === value}
            onClick={() => onChange(kebab)}
            className={cn(
              'flex size-8 items-center justify-center rounded-sm text-muted-foreground transition-colors hover:bg-muted hover:text-foreground',
              kebab === value && 'bg-primary/15 text-primary hover:bg-primary/15 hover:text-primary',
            )}
          >
            <Ico className="size-4" />
          </button>
        ))}
        {hits.length === 0 && (
          <div className="col-span-8 py-4 text-center text-xs text-muted-foreground">无匹配图标</div>
        )}
      </div>
      {/* 命中裁剪提示 + 当前值回显（选中项可能不在可视区） */}
      <div className="text-[10px] text-muted-foreground">
        {ALL.length > MAX_SHOWN && hits.length === MAX_SHOWN
          ? `显示前 ${MAX_SHOWN} / ${ALL.length} 个，搜索缩小范围；`
          : ''}
        当前：{value || '未选择'}
      </div>
    </div>
  );
}
