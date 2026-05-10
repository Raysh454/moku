import React from "react";
import { HashRouter, Routes, Route, Navigate } from "react-router-dom";

import { ProjectProvider } from "../context/ProjectContext";

import ProjectSelectPage from "../pages/ProjectSelect/ProjectSelectPage";
import ProjectCreatePage from "../pages/ProjectCreate/ProjectCreatePage";
import WorkspacePage from "../pages/Workspace/WorkspacePage";

function App() {
  return (
    <ProjectProvider>
      <HashRouter>
        <main className="min-h-screen bg-bg text-gray-200 selection:bg-accent selection:text-bg">
          <Routes>
            <Route path="/" element={<ProjectSelectPage />} />

            <Route path="/create" element={<ProjectCreatePage />} />

            <Route path="/workspace" element={<WorkspacePage />} />

            <Route path="*" element={<Navigate to="/" replace />} />
          </Routes>
        </main>
      </HashRouter>
    </ProjectProvider>
  );
}

export default App;
