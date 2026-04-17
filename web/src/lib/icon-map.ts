import {
  Archive,
  BookOpen,
  Bot,
  Brain,
  Briefcase,
  Building2,
  CheckCircle,
  Cog,
  Flag,
  FolderTree,
  Headset,
  Home,
  List,
  MessageSquare,
  Settings,
  Server,
  Timer,
  User,
  Users,
  Shield,
  Menu,
  Sliders,
  Wrench,
  FileText,
  FileBadge,
  Folder,
  LayoutDashboard,
  LayoutGrid,
  FolderOpen,
  MousePointerClick,
  KeyRound,
  Plug,
  UserCog,
  Database,
  Bell,
  BarChart3,
  Globe,
  Lock,
  Mail,
  Megaphone,
  Monitor,
  Clock,
  ClipboardList,
  Fingerprint,
  Package,
  Ticket,
  Activity,
  GitBranch,
  Network,
  type LucideIcon,
} from 'lucide-react';

const icons = {
  Archive,
  BookOpen,
  Bot,
  Brain,
  Briefcase,
  CheckCircle,
  Cog,
  Flag,
  FolderTree,
  Headset,
  Home,
  List,
  MessageSquare,
  Settings,
  Server,
  Timer,
  User,
  Users,
  Shield,
  Menu,
  Sliders,
  Wrench,
  FileText,
  Folder,
  FolderOpen,
  LayoutDashboard,
  LayoutGrid,
  MousePointerClick,
  KeyRound,
  Plug,
  UserCog,
  Database,
  Bell,
  BarChart3,
  Globe,
  Lock,
  Mail,
  Megaphone,
  Monitor,
  Clock,
  ClipboardList,
  Fingerprint,
  Package,
  Building2,
  FileBadge,
  Ticket,
  Activity,
  GitBranch,
  Network,
};

function normalizeIconName(name: string): string {
  return name.replace(/[^a-z0-9]/gi, '').toLowerCase();
}

const iconMap: Record<string, LucideIcon> = Object.entries(icons).reduce<
  Record<string, LucideIcon>
>((acc, [name, icon]) => {
  acc[name] = icon;
  acc[normalizeIconName(name)] = icon;
  return acc;
}, {});

/** Fallback icon per menu type */
const typeFallback: Record<string, LucideIcon> = {
  directory: Folder,
  menu: FileText,
  button: MousePointerClick,
};

/**
 * Resolve a menu item's icon by name.
 * Falls back to a type-specific icon, then to FileText.
 */
export function getIcon(name: string | undefined, type?: string): LucideIcon {
  if (name) {
    const normalizedName = normalizeIconName(name);
    if (iconMap[name]) return iconMap[name];
    if (iconMap[normalizedName]) return iconMap[normalizedName];
  }
  if (type && typeFallback[type]) return typeFallback[type];
  return FileText;
}
