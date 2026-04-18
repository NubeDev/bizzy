import { Routes, Route, Navigate } from 'react-router-dom'
import { AppShell } from './components/layout/app-shell'
import { StoreHomePage } from './pages/store-home'
import { AppDetailPage } from './pages/app-detail'
import { MyAppsPage } from './pages/my-apps'
import { AppEditorPage } from './pages/app-editor'
import { CreateAppPage } from './pages/create-app'
import { ChatPage } from './pages/chat'
import { WorkshopPage } from './pages/workshop'
import { PluginsPage } from './pages/plugins'
import { FlowsPage } from './pages/flows'
import { FlowEditorPage } from './pages/flow-editor'

function App() {
  return (
    <Routes>
      <Route element={<AppShell />}>
        <Route path="/" element={<StoreHomePage />} />
        <Route path="/chat" element={<ChatPage />} />
        <Route path="/apps/:id" element={<AppDetailPage />} />
        <Route path="/my-apps" element={<MyAppsPage />} />
        <Route path="/my-apps/create" element={<CreateAppPage />} />
        <Route path="/my-apps/create/:id" element={<CreateAppPage />} />
        <Route path="/my-apps/:id/edit" element={<AppEditorPage />} />
        <Route path="/my-apps/:id/builder" element={<CreateAppPage />} />
        <Route path="/workshop" element={<WorkshopPage />} />
        <Route path="/my-apps/:id/workshop" element={<WorkshopPage />} />
        <Route path="/plugins" element={<PluginsPage />} />
        <Route path="/flows" element={<FlowsPage />} />
        <Route path="/flows/:id" element={<FlowEditorPage />} />
      </Route>
      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  )
}

export default App
