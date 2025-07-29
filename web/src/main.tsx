import React from "react";
import ReactDOM from "react-dom/client";
import { createBrowserRouter, RouterProvider } from "react-router-dom";
import App from "./App";
import ManagerPage from "./pages/ManagerPage";
import SessionsPage from "./pages/SessionsPage";
import ThreadSessionsPage from "./pages/ThreadSessionsPage";
import ThreadsPage from "./pages/ThreadsPage";
import "../styles/index.css";

// Dynamically determine basename based on current URL
const getBasename = () => {
  const pathname = window.location.pathname;
  // If we're under /web, use /web as basename
  if (pathname.startsWith("/web")) {
    return "/web";
  }
  // Otherwise, use root
  return "/";
};

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
    basename: getBasename(),
  },
);

ReactDOM.createRoot(document.getElementById("root") as HTMLElement).render(
  <React.StrictMode>
    <RouterProvider router={router} />
  </React.StrictMode>,
);
