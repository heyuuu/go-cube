import { Navigate, Route, Routes } from 'react-router';

import { Layout } from './components/layout';
import { ConfigPage } from './pages/config';
import { NotFoundPage } from './pages/errors/not-found';
import { ProjectsPage } from './pages/projects';

function App() {
  return (
    <Routes>
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
