import { Navigate, Route, Routes } from 'react-router';

import { Layout } from './components/layout';
import { ConfigPage } from './pages/config';
import { NotFoundPage } from './pages/errors/not-found';
import { MdPage } from './pages/md';
import { ProjectsPage } from './pages/projects';
import { WorkbenchPage } from './pages/workbench';

function App() {
  return (
    <Routes>
      {/* md 渲染页独立于主应用 Layout：文档查看器，不带业务侧栏 */}
      {/* md 渲染页独立于主应用 Layout：文档查看器，不带业务侧栏 */}
      <Route path="/md" element={<MdPage />} />
      {/* 工作台独立于主应用 Layout：以 git 目录为输入的聚合界面，自带整体布局 */}
      <Route path="/workbench" element={<WorkbenchPage />} />
      <Route path="/" element={<Layout />}>
        <Route index element={<Navigate to="/projects" replace />} />
        <Route path="projects" element={<ProjectsPage />} />
        <Route path="config" element={<ConfigPage />} />
        <Route path="*" element={<NotFoundPage />} />
      </Route>
    </Routes>
  );
}

export default App;
