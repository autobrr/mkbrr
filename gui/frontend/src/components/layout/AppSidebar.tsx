import { useState, useEffect } from 'react';
import { NavLink } from 'react-router-dom';
import { FilePlus, FileSearch, FileCheck, FileEdit, Settings, Moon, Sun, Palette, ExternalLink } from 'lucide-react';
import { cn } from '@/lib/utils';
import { Button } from '@/components/ui/button';
import { Separator } from '@/components/ui/separator';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { useTheme, ThemeName } from '@/hooks/useTheme';
import { GetVersion } from '../../../wailsjs/go/main/App';
import { BrowserOpenURL } from '../../../wailsjs/runtime/runtime';

const navItems = [
  { to: '/', icon: FilePlus, label: 'Create' },
  { to: '/inspect', icon: FileSearch, label: 'Inspect' },
  { to: '/check', icon: FileCheck, label: 'Check' },
  { to: '/modify', icon: FileEdit, label: 'Modify' },
];

export function AppSidebar() {
  const { mode, theme, toggleMode, setTheme, availableThemes } = useTheme();
  const [version, setVersion] = useState('');

  useEffect(() => {
    GetVersion().then(setVersion).catch(() => setVersion('dev'));
  }, []);

  return (
    <aside className="flex h-screen w-56 flex-col border-r border-sidebar-border bg-sidebar">
      <div className="flex h-14 items-center px-4">
        <h1 className="text-lg font-semibold text-sidebar-foreground">mkbrr</h1>
      </div>

      <Separator />

      <nav className="flex-1 space-y-1 p-2">
        {navItems.map((item) => (
          <NavLink
            key={item.to}
            to={item.to}
            className={({ isActive }: { isActive: boolean }) =>
              cn(
                'flex items-center gap-3 rounded-md px-3 py-2 text-sm font-medium transition-colors',
                isActive
                  ? 'bg-sidebar-accent text-sidebar-accent-foreground'
                  : 'text-sidebar-foreground/70 hover:bg-sidebar-accent/50 hover:text-sidebar-foreground'
              )
            }
          >
            <item.icon className="h-4 w-4" />
            {item.label}
          </NavLink>
        ))}
      </nav>

      <Separator />

      <div className="p-2 space-y-1">
        <NavLink
          to="/settings"
          className={({ isActive }: { isActive: boolean }) =>
            cn(
              'flex items-center gap-3 rounded-md px-3 py-2 text-sm font-medium transition-colors',
              isActive
                ? 'bg-sidebar-accent text-sidebar-accent-foreground'
                : 'text-sidebar-foreground/70 hover:bg-sidebar-accent/50 hover:text-sidebar-foreground'
            )
          }
        >
          <Settings className="h-4 w-4" />
          Settings
        </NavLink>

        <div className="flex items-center gap-2 px-3 py-2">
          <Palette className="h-4 w-4 text-sidebar-foreground/70" />
          <Select value={theme} onValueChange={(value) => setTheme(value as ThemeName)}>
            <SelectTrigger className="h-8 flex-1 text-xs">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {availableThemes.map((t) => (
                <SelectItem key={t.name} value={t.name}>
                  {t.displayName}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>

        <Button
          variant="ghost"
          size="sm"
          className="w-full justify-start gap-3 px-3 text-sidebar-foreground/70 hover:bg-sidebar-accent/50 hover:text-sidebar-foreground"
          onClick={toggleMode}
        >
          {mode === 'dark' ? (
            <>
              <Sun className="h-4 w-4" />
              Light Mode
            </>
          ) : (
            <>
              <Moon className="h-4 w-4" />
              Dark Mode
            </>
          )}
        </Button>
      </div>

      <div className="p-3 space-y-1">
        <Button
          variant="ghost"
          size="sm"
          className="w-full justify-start gap-3 px-3 text-sidebar-foreground/70 hover:bg-sidebar-accent/50 hover:text-sidebar-foreground h-8 text-xs"
          onClick={() => BrowserOpenURL('https://mkbrr.com')}
        >
          <ExternalLink className="h-3.5 w-3.5" />
          Documentation
        </Button>
        <div className="flex items-center justify-between px-3 py-1 text-xs text-sidebar-foreground/50">
          <span>{version || ''}</span>
          <span>GPL-2.0</span>
        </div>
      </div>
    </aside>
  );
}
