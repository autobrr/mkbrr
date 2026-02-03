import { Outlet } from 'react-router-dom';
import { AppSidebar } from './AppSidebar';

export function Layout() {
  return (
    <div className="flex h-screen overflow-hidden">
      <AppSidebar />
      <main className="flex-1 overflow-auto bg-background">
        <Outlet />
      </main>
    </div>
  );
}
