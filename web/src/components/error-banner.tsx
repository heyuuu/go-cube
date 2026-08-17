// 错误横幅：React Query error 分支的统一展示
export function ErrorBanner({ message }: { message: string }) {
  return (
    <div className="mx-6 mb-3 rounded-md border border-destructive/30 bg-destructive/10 px-3 py-2 text-xs text-destructive">
      {message}
    </div>
  );
}
