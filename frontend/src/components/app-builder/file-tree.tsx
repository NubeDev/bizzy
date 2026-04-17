/**
 * File tree panel — shows the virtual project structure.
 * Groups files by directory (tools/, prompts/, ui/).
 */
import { Folder, FileCode, FileJson, FileText, Settings, Plus, Component } from "lucide-react"
import { groupFiles, type AppFile } from "./types"
import { Button } from "@/components/ui/button"

interface Props {
  files: AppFile[]
  selectedPath: string | null
  onSelect: (path: string) => void
}

const TYPE_ICONS: Record<string, typeof FileCode> = {
  yaml: Settings,
  js: FileCode,
  json: FileJson,
  md: FileText,
  tsx: Component,
}

export function FileTree({ files, selectedPath, onSelect }: Props) {
  const groups = groupFiles(files)
  const dirs = Object.keys(groups).sort()

  return (
    <div className="h-full flex flex-col">
      <div className="px-3 py-2.5 border-b border-border flex items-center justify-between">
        <span className="text-xs font-mono font-medium text-muted-foreground uppercase tracking-wider">Files</span>
        <span className="text-[10px] text-muted-foreground">{files.length}</span>
      </div>

      <div className="flex-1 overflow-y-auto py-1">
        {dirs.map(dir => {
          const dirFiles = groups[dir]
          if (dir === "/") {
            return dirFiles.map(f => <FileRow key={f.path} file={f} selected={selectedPath === f.path} onSelect={onSelect} />)
          }
          return (
            <div key={dir}>
              <div className="flex items-center gap-1.5 px-3 py-1.5 text-[11px] font-mono text-muted-foreground uppercase tracking-wider">
                <Folder size={12} />
                {dir}/
              </div>
              {dirFiles.map(f => (
                <FileRow key={f.path} file={f} selected={selectedPath === f.path} onSelect={onSelect} indent />
              ))}
            </div>
          )
        })}

        {files.length === 0 && (
          <div className="px-3 py-8 text-center text-xs text-muted-foreground">
            No files yet. Describe your app in the chat to generate the project.
          </div>
        )}
      </div>
    </div>
  )
}

function FileRow({ file, selected, onSelect, indent }: {
  file: AppFile; selected: boolean; onSelect: (path: string) => void; indent?: boolean
}) {
  const Icon = TYPE_ICONS[file.type] || FileText
  const fileName = file.path.split("/").pop() || file.path

  return (
    <button
      onClick={() => onSelect(file.path)}
      className={`w-full flex items-center gap-2 px-3 py-1.5 text-left text-[12px] font-mono transition-colors ${
        indent ? "pl-6" : ""
      } ${
        selected
          ? "bg-accent text-foreground"
          : "text-muted-foreground hover:bg-accent/50 hover:text-foreground"
      }`}
    >
      <Icon size={12} className="shrink-0" />
      <span className="truncate">{fileName}</span>
      {file.dirty && <span className="w-1.5 h-1.5 rounded-full bg-blue-400 shrink-0" />}
    </button>
  )
}
