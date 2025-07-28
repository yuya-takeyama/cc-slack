import React from "react";
import ReactDOM from "react-dom/client";
import { createBrowserRouter, RouterProvider } from "react-router-dom";
import App from "./App";
import SessionsPage from "./pages/SessionsPage";
import ThreadSessionsPage from "./pages/ThreadSessionsPage";
import ThreadsPage from "./pages/ThreadsPage";
import ManagerPage from "./pages/ManagerPage";
import "../styles/index.css";

const router = createBrowserRouter(
  [
    {
      path: "/",
      element: <App />,
      children: [
        {
          index: true,
          element: <ThreadsPage />,
        },
        {
          path: "sessions",
          element: <SessionsPage />,
        },
        {
          path: "threads/:threadId/sessions",
          element: <ThreadSessionsPage />,
        },
        {
          path: "manager",
          element: <ManagerPage />,
        },
      ],
    },
  ],
  {
    basename: "/web",
  },
);

ReactDOM.createRoot(document.getElementById("root") as HTMLElement).render(
  <React.StrictMode>
    <RouterProvider router={router} />
  </React.StrictMode>,
);
