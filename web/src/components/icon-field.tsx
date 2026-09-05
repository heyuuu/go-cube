// icon 声明编辑字段：类型切换（lucide / image）+ lucide 搜索点选 + 路径/URL 提取 + PNG 上传。
// 从 Opener 表单的 inline 块抽出泛化（opener / scanRules / forge 共用）：
//   - allowEmpty=true：icon 可选（scanRules / forge），显示「清除」按钮，无默认值；
//   - allowEmpty=false：icon 必有值（opener），切回 lucide 时回落 defaultLucide。
import { useState } from 'react';

import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import type { IconDecl } from '@/lib/icon';
import { renderIcon } from '@/lib/icon';
import { useIconExtract } from '@/queries/icon';

import { LucideIconPicker } from './lucide-icon-picker';

interface IconFieldProps {
  value: IconDecl | undefined;
  onChange: (v: IconDecl | undefined) => void;
  allowEmpty?: boolean;
  /** allowEmpty=false 时切回 lucide 的回落图（与后端默认值一致，如 opener 的 app-window-mac） */
  defaultLucide?: string;
}

export function IconField({ value, onChange, allowEmpty = false, defaultLucide }: IconFieldProps) {
  const extract = useIconExtract();
  const [source, setSource] = useState('');
  const type = value?.type ?? 'lucide';

  const switchType = (t: IconDecl['type']) => {
    if (t === 'lucide') {
      // 切 lucide 回落已有 lucide 值或默认图，避免「未选择」态（必填场景）
      const lucideValue = value?.type === 'lucide' ? value.value : (defaultLucide ?? '');
      onChange(lucideValue ? { type: 'lucide', value: lucideValue } : undefined);
    } else {
      onChange({ type: 'image', value: value?.type === 'image' ? value.value : '' });
    }
  };

  const onUpload = (file: File) => {
    const reader = new FileReader();
    reader.onload = () => {
      // dataURL 去掉前缀，存纯 base64
      onChange({ type: 'image', value: String(reader.result).replace(/^data:[^,]*,/, '') });
    };
    reader.readAsDataURL(file);
  };

  return (
    <div className="flex flex-col gap-1">
      <div className="flex items-center gap-2">
        {(['lucide', 'image'] as const).map((t) => (
          <Button key={t} size="sm" variant={type === t ? 'default' : 'outline'} onClick={() => switchType(t)}>
            {t}
          </Button>
        ))}
        {allowEmpty && value && (
          <Button size="sm" variant="ghost" onClick={() => onChange(undefined)}>
            清除
          </Button>
        )}
        {value && <span className="ml-1">{renderIcon(value, null)}</span>}
      </div>
      {type === 'lucide' && (
        <LucideIconPicker
          value={value?.type === 'lucide' ? value.value : ''}
          onChange={(v) => onChange({ type: 'lucide', value: v })}
        />
      )}
      {type === 'image' && (
        <div className="flex flex-col gap-2">
          {value?.type === 'image' && value.value && (
            <img src={`data:image/png;base64,${value.value}`} alt="icon 预览" className="size-8 rounded" />
          )}
          <div className="flex items-center gap-2">
            <Input
              value={source}
              onChange={(e) => setSource(e.target.value)}
              placeholder="/Applications/Xxx.app、图片路径或 https://gitee.com/favicon.ico"
            />
            <Button
              size="sm"
              variant="outline"
              disabled={!source || extract.isPending}
              onClick={() => extract.mutate(source, { onSuccess: (v) => onChange({ type: 'image', value: v }) })}
            >
              提取
            </Button>
          </div>
          <input
            type="file"
            accept="image/png"
            onChange={(e) => e.target.files?.[0] && onUpload(e.target.files[0])}
            className="text-xs"
          />
        </div>
      )}
      {extract.error && <span className="text-xs text-destructive">提取失败：{extract.error.message}</span>}
    </div>
  );
}
