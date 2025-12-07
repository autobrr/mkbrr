import { BrowserRouter, Routes, Route } from 'react-router-dom';
import { Layout } from '@/components/layout/Layout';
import { CreatePage } from '@/pages/Create';
import { InspectPage } from '@/pages/Inspect';
import { CheckPage } from '@/pages/Check';
import { ModifyPage } from '@/pages/Modify';
import { SettingsPage } from '@/pages/Settings';

function App() {
  return (
    <BrowserRouter>
      <Routes>
        <Route element={<Layout />}>
          <Route path="/" element={<CreatePage />} />
          <Route path="/inspect" element={<InspectPage />} />
          <Route path="/check" element={<CheckPage />} />
          <Route path="/modify" element={<ModifyPage />} />
          <Route path="/settings" element={<SettingsPage />} />
        </Route>
      </Routes>
    </BrowserRouter>
  );
}

export default App;
