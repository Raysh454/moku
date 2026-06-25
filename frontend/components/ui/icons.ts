/**
 * Curated icon facade over lucide-react.
 *
 * Every component imports icons from here rather than from `lucide-react`
 * directly, so the icon set is swappable in one place and the app stops
 * hand-inlining `<svg>` paths. Add an icon here before using it elsewhere.
 */
export type { LucideIcon as IconComponent } from "lucide-react";

export {
  // chevrons / disclosure
  ChevronRight,
  ChevronDown,
  ChevronLeft,
  ChevronsLeft,
  ChevronsRight,
  // files / explorer
  Folder,
  FolderOpen,
  File as FileIcon,
  FileText,
  FileCode,
  FileJson,
  Image as ImageIcon,
  Braces,
  Globe,
  ListTree,
  // actions
  Settings,
  Play,
  RefreshCw,
  Plus,
  Trash2,
  MoreVertical,
  Search,
  X,
  Check,
  ArrowLeft,
  ArrowLeftRight,
  Download,
  // domain / security
  GitCompare,
  ShieldAlert,
  ShieldCheck,
  ScanLine,
  Boxes,
  FlaskConical,
  Eye,
  Code2,
  AlertTriangle,
  CircleDot,
  Loader2,
} from "lucide-react";
