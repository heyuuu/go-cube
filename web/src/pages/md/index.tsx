import Markdown from 'react-markdown';
import { useSearchParams } from 'react-router';
import remarkGfm from 'remark-gfm';

import { ErrorBanner } from '@/components/error-banner';
import { useMdContent } from '@/queries/md';

// md 渲染页：独立于主应用 Layout（文档查看器，不带业务侧栏）。
// 路由 /md?path=<abs>，由 `cube md` 命令打开；渲染全在前端（后端只给原文）。
export function MdPage() {
  const [searchParams] = useSearchParams();
  const path = searchParams.get('path') ?? '';
  const q = useMdContent(path);
  const fileName = path.split('/').pop() || '未命名';

  if (!path) {
    return (
      <main className="mx-auto max-w-3xl px-6 py-10">
        <ErrorBanner message="缺少 path 参数：请通过 cube md <file> 打开本页" />
      </main>
    );
  }

  return (
    <main className="mx-auto max-w-3xl px-6 py-8">
      <header className="mb-6 border-b pb-4">
        <h1 className="text-lg font-semibold">{fileName}</h1>
        <div className="mt-1 font-mono text-xs text-muted-foreground" title={path}>
          {path}
        </div>
      </header>

      {q.isPending && <div className="text-xs text-muted-foreground">加载中…</div>}
      {q.error && <ErrorBanner message={`读取失败：${q.error.message}`} />}
      {q.data && (
        <article className="prose prose-sm dark:prose-invert max-w-none">
          <Markdown remarkPlugins={[remarkGfm]}>{q.data.content}</Markdown>
        </article>
      )}
    </main>
  );
}
