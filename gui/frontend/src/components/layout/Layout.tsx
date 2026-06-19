import { Outlet, useLocation } from 'react-router-dom';
import { AppSidebar } from './AppSidebar';
import { ErrorBoundary } from '@/components/ErrorBoundary';

export function Layout() {
  const location = useLocation();
  return (
    <div className="flex h-screen overflow-hidden">
      <AppSidebar />
      <main className="flex-1 overflow-auto bg-background">
        {/* Key by route so a crash on one page clears when the user navigates
            elsewhere (error boundaries don't reset on route change by default). */}
        <ErrorBoundary key={location.pathname}>
          <Outlet />
        </ErrorBoundary>
      </main>
    </div>
  );
}
