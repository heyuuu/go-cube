import { Navigate, Route, Routes } from 'react-router';

import { Layout } from './components/layout';
import { ConfigPage } from './pages/config';
import { NotFoundPage } from './pages/errors/not-found';
import { MdPage } from './pages/md';
import { ProjectsPage } from './pages/projects';

function App() {
  return (
    <Routes>
      {/* md 渲染页独立于主应用 Layout：文档查看器，不带业务侧栏 */}
      <Route path="/md" element={<MdPage />} />
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
