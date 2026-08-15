import { Link } from 'react-router';

export function NotFoundPage() {
  return (
    <div className="p-6">
      <h1 className="text-lg font-semibold">404</h1>
      <p className="mt-1 text-xs text-muted-foreground">页面不存在</p>
      <Link to="/projects" className="mt-4 inline-block text-xs text-primary hover:underline">
        回项目列表
      </Link>
    </div>
  );
}
