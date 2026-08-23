import { EditorState, Compartment } from '@codemirror/state';
import { oneDark } from '@codemirror/theme-one-dark';
import { EditorView } from '@codemirror/view';
import { basicSetup } from 'codemirror';
import { useEffect, useRef } from 'react';

import { useTheme } from '@/hooks/use-theme';
import { languageForFile } from '@/lib/cm-lang';
import { cn } from '@/lib/utils';

// CodeMirror6 只读/可编辑统一封装（提案 1012）。
// 受控形态：value 变化重建文档；onChange 仅在可编辑时回调。
// readOnly 用 Compartment 切换避免重建视图（draft 切换编辑态不丢光标位置）。
// 主题联动全局明暗（useTheme）：暗色用 One Dark，亮色用 basicSetup 默认浅色；
// 同样走 Compartment 热切换，不重建视图。diff 面板（1013）复用本组件。
export function CodeEditor({
  value,
  file,
  readOnly,
  onChange,
  className,
}: {
  value: string;
  file: string;
  readOnly: boolean;
  onChange?: (next: string) => void;
  className?: string;
}) {
  const holderRef = useRef<HTMLDivElement>(null);
  const viewRef = useRef<EditorView | null>(null);
  const readOnlyComp = useRef(new Compartment());
  const themeComp = useRef(new Compartment());
  const onChangeRef = useRef(onChange);
  const valueRef = useRef(value);
  const fileRef = useRef(file);
  const readOnlyRef = useRef(readOnly);
  const theme = useTheme();
  const themeRef = useRef(theme);
  useEffect(() => {
    onChangeRef.current = onChange;
    valueRef.current = value;
    fileRef.current = file;
    readOnlyRef.current = readOnly;
    themeRef.current = theme;
  });

  useEffect(() => {
    if (!holderRef.current) return;
    const view = new EditorView({
      state: EditorState.create({
        doc: valueRef.current,
        extensions: [
          basicSetup,
          themeComp.current.of(themeRef.current === 'dark' ? oneDark : []),
          languageForFile(fileRef.current),
          readOnlyComp.current.of(EditorState.readOnly.of(readOnlyRef.current)),
          EditorView.lineWrapping,
          EditorView.updateListener.of((update) => {
            if (update.docChanged) {
              onChangeRef.current?.(update.state.doc.toString());
            }
          }),
        ],
      }),
      parent: holderRef.current,
    });
    viewRef.current = view;
    return () => {
      view.destroy();
      viewRef.current = null;
    };
    // 视图只在挂载时创建一次；file/language 变化由调用方通过 key={file} 重挂载触发
    // （语言扩展不可动态换）；value / readOnly 的变化由下方两个同步效应处理。
    // 初值经 ref 传递，effect 不引用任何响应式值，空依赖是终态。
  }, []);

  // 外部 value 变化（切换文件 / 放弃编辑）时同步文档
  useEffect(() => {
    const view = viewRef.current;
    if (view && view.state.doc.toString() !== value) {
      view.dispatch({ changes: { from: 0, to: view.state.doc.length, insert: value } });
    }
  }, [value]);

  // 编辑态切换
  useEffect(() => {
    viewRef.current?.dispatch({
      effects: readOnlyComp.current.reconfigure(EditorState.readOnly.of(readOnly)),
    });
  }, [readOnly]);

  // 全局明暗切换：热切换编辑器主题，不重建视图
  useEffect(() => {
    viewRef.current?.dispatch({
      effects: themeComp.current.reconfigure(theme === 'dark' ? oneDark : []),
    });
  }, [theme]);

  return <div ref={holderRef} className={cn('overflow-auto text-[13px]', className)} />;
}
