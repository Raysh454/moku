import React, { Suspense } from "react";
import { HashRouter, Routes, Route, Navigate } from "react-router-dom";

import { ProjectProvider } from "../context/ProjectContext";
import { EditorProvider } from "../context/EditorContext";
import { NotificationProvider } from "../context/NotificationContext";
import { JobEventProvider } from "../context/JobEventContext";

import ProjectSelectPage from "../pages/ProjectSelect/ProjectSelectPage";
import ProjectCreatePage from "../pages/ProjectCreate/ProjectCreatePage";
import { NotificationViewport } from "../components/common/NotificationViewport";

// The workspace pulls in the heavy diff/tree engines (Shiki); lazy-load it so
// the project routes stay lightweight.
const WorkspacePage = React.lazy(() => import("../pages/Workspace/WorkspacePage"));

function App() {
  return (
    <NotificationProvider>
      <JobEventProvider>
        <ProjectProvider>
          <EditorProvider>
            <HashRouter>
              <main className="min-h-screen bg-bg text-gray-200 selection:bg-accent selection:text-bg">
              <Routes>
                <Route path="/" element={<ProjectSelectPage />} />

                <Route path="/create" element={<ProjectCreatePage />} />

                <Route
                  path="/workspace"
                  element={
                    <Suspense
                      fallback={
                        <div className="flex h-screen items-center justify-center bg-bg text-sm text-helper">
                          Loading workspace…
                        </div>
                      }
                    >
                      <WorkspacePage />
                    </Suspense>
                  }
                />

                <Route path="*" element={<Navigate to="/" replace />} />
              </Routes>
              <NotificationViewport />
              </main>
            </HashRouter>
          </EditorProvider>
        </ProjectProvider>
      </JobEventProvider>
    </NotificationProvider>
  );
}


export default App;
