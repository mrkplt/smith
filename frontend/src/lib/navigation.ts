import {
  ArchiveOutline,
  AdjustmentsHorizontalOutline,
  FileLinesOutline,
  GridOutline
} from 'flowbite-svelte-icons';

export type ShellNavItem = {
  id: 'pods' | 'documents' | 'tasks' | 'feature-capability' | 'settings';
  label: string;
  href: '/pods' | '/documents' | '/tasks' | '/feature-capability' | '/settings';
  icon: typeof GridOutline;
};

export const shellRuntimeNav: ShellNavItem[] = [
  { id: 'pods', label: 'Pods', href: '/pods', icon: GridOutline },
  { id: 'documents', label: 'Documents', href: '/documents', icon: FileLinesOutline },
  { id: 'tasks', label: 'Tasks', href: '/tasks', icon: FileLinesOutline },
  { id: 'feature-capability', label: 'Feature Flow', href: '/feature-capability', icon: ArchiveOutline }
];

export const shellConfigurationNav: ShellNavItem[] = [
  { id: 'settings', label: 'Settings', href: '/settings', icon: AdjustmentsHorizontalOutline }
];

export const shellPrimaryNav: ShellNavItem[] = [
  ...shellRuntimeNav,
  ...shellConfigurationNav
];
