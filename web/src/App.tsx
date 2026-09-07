import { Navigate, Route, Routes } from 'react-router';

import { Layout } from './components/layout';
import { useTheme } from './hooks/use-theme';
import { NotFoundPage } from './pages/errors/not-found';
import { ForgesPage } from './pages/forges';
import { MdPage } from './pages/md';
import { ProjectsPage } from './pages/projects';
import { SettingsPage } from './pages/settings';
import { WorkbenchPage } from './pages/workbench';

function App() {
  // 挂在 App 根上（而非 Layout），保证壳外的 /md 独立路由也有 Cmd+D 切主题
  useTheme();

  return (
    <Routes>
      {/* md 渲染页独立于主应用 Layout：文档查看器（如 `cube md` 新 tab 打开），不带业务侧栏 */}
      <Route path="/md" element={<MdPage />} />
      <Route path="/" element={<Layout />}>
        <Route index element={<Navigate to="/projects" replace />} />
        <Route path="projects" element={<ProjectsPage />} />
        <Route path="forges" element={<ForgesPage />} />
        {/* 工作台挂进全局壳（提案 1023）：Layout 按此前缀切铺满型 main，不吃 max-w 收敛 */}
        <Route path="workbench" element={<WorkbenchPage />} />
        <Route path="settings" element={<SettingsPage />} />
        <Route path="*" element={<NotFoundPage />} />
      </Route>
    </Routes>
  );
}

export default App;
