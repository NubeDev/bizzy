import { clsx, type ClassValue } from "clsx"
import { twMerge } from "tailwind-merge"

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}

export function formatDate(dateStr: string): string {
  return new Date(dateStr).toLocaleDateString('en-US', {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
  })
}

export function categoryLabel(slug: string): string {
  const labels: Record<string, string> = {
    'iot-devices': 'IoT & Devices',
    analytics: 'Analytics',
    devops: 'DevOps',
    marketing: 'Marketing',
    design: 'Design',
    utilities: 'Utilities',
    integrations: 'Integrations',
    automation: 'Automation',
  }
  return labels[slug] || slug
}
