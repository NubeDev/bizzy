import { Routes, Route, Navigate } from 'react-router-dom'
import { AppShell } from './components/layout/app-shell'
import { StoreHomePage } from './pages/store-home'
import { AppDetailPage } from './pages/app-detail'
import { MyAppsPage } from './pages/my-apps'
import { AppEditorPage } from './pages/app-editor'
import { CreateAppPage } from './pages/create-app'
import { ChatPage } from './pages/chat'

function App() {
  return (
    <Routes>
      <Route element={<AppShell />}>
        <Route path="/" element={<StoreHomePage />} />
        <Route path="/chat" element={<ChatPage />} />
        <Route path="/apps/:id" element={<AppDetailPage />} />
        <Route path="/my-apps" element={<MyAppsPage />} />
        <Route path="/my-apps/create" element={<CreateAppPage />} />
        <Route path="/my-apps/:id/edit" element={<AppEditorPage />} />
      </Route>
      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  )
}

export default App
